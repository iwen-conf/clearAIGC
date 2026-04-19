# Naturalize — 安全与多租户

## 1. 认证

### 1.1 JWT Bearer Token（Phase 5）

```go
func JWTAuthMiddleware(secret []byte) gin.HandlerFunc {
    return func(c *gin.Context) {
        header := c.GetHeader("Authorization")
        if !strings.HasPrefix(header, "Bearer ") {
            c.AbortWithStatus(401)
            return
        }
        
        token := header[7:]
        claims, err := validateToken(token, secret)
        if err != nil {
            c.AbortWithStatus(401)
            return
        }
        
        c.Set("user_id", claims.UserID)
        c.Set("tenant_id", claims.TenantID)
        c.Set("roles", claims.Roles)
        c.Next()
    }
}
```

| 端点 | 说明 | Token 生命周期 |
| :--- | :--- | :--- |
| `POST /auth/login` | 获取 Access + Refresh Token | Access: 15m, Refresh: 7d |
| `POST /auth/refresh` | 用 Refresh Token 换新 Access Token | — |

Phase 1-4 可跳过认证中间件。

### 1.2 API Key 认证（轻量模式）

用于 Webhook 回调、CLI 工具和自动化集成：

```
X-API-Key: cak_live_xxxxxxxxxxxxxxxxxxxxx
```

- API Key 以 `cak_live_` 为前缀，便于识别和扫描泄漏
- 存储：`bcrypt(api_key)` 存 DB，原文仅在创建时展示一次
- 支持设置过期时间和权限范围

## 2. RBAC 权限模型（Phase 5）

| 角色 | 权限 | 说明 |
| :--- | :--- | :--- |
| `viewer` | 查看 Session、下载导出文件 | 只读访问 |
| `editor` | viewer + 创建 Session、启动/暂停/恢复 Round | 标准用户 |
| `admin` | editor + 管理配置、Provider、Webhook、用户 | 管理员 |
| `owner` | admin + 删除租户、查看审计日志、管理 API Key | 租户所有者 |

```go
func RequireRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRoles := c.GetStringSlice("roles")
        for _, required := range roles {
            if slices.Contains(userRoles, required) {
                c.Next()
                return
            }
        }
        c.AbortWithStatus(403)
    }
}

// 使用示例
router.DELETE("/sessions/:id", RequireRole("editor", "admin", "owner"), handler.DeleteSession)
router.PUT("/config", RequireRole("admin", "owner"), handler.UpdateConfig)
```

## 3. 多租户数据隔离（Phase 5）

PostgreSQL Row-Level Security (RLS) 实现数据隔离：

```sql
ALTER TABLE sessions ADD COLUMN tenant_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';
ALTER TABLE rounds ADD COLUMN tenant_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000';

ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE rounds ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON sessions
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::UUID);

CREATE POLICY tenant_isolation ON rounds
    FOR ALL
    USING (tenant_id = current_setting('app.current_tenant')::UUID);
```

Middleware 自动注入租户上下文：

```go
func TenantMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := c.GetString("tenant_id")
        if tenantID == "" {
            c.AbortWithStatus(403)
            return
        }
        
        // 在数据库连接上设置租户上下文
        c.Set("db", c.MustGet("db").(*gorm.DB).Session(&gorm.Session{
            NewDB: true,
        }).Exec("SET app.current_tenant = ?", tenantID))
        
        c.Next()
    }
}
```

## 4. 输入验证

```go
func validateCreateSession(req CreateSessionRequest) error {
    // 文件大小限制 (50MB)
    if req.FileSize > 50*1024*1024 {
        return fmt.Errorf("file too large: %d bytes (max 50MB)", req.FileSize)
    }
    
    // 允许的文件格式 (白名单)
    if !slices.Contains([]string{"txt", "docx"}, req.FileFormat) {
        return fmt.Errorf("invalid file format: %s", req.FileFormat)
    }
    
    // Prompt Profile 白名单
    if !slices.Contains([]string{"cn", "en"}, req.PromptProfile) {
        return fmt.Errorf("invalid prompt profile: %s", req.PromptProfile)
    }
    
    // chunk_limit 范围
    if req.ChunkLimit < 100 || req.ChunkLimit > 2000 {
        return fmt.Errorf("chunk_limit out of range: %d (100-2000)", req.ChunkLimit)
    }
    
    // 文件名清理 (防止路径遍历)
    if strings.Contains(req.FileName, "..") || strings.Contains(req.FileName, "/") {
        return fmt.Errorf("invalid file name")
    }
    
    return nil
}
```

### DOCX 文件安全

```go
func validateDocx(path string) error {
    r, err := zip.OpenReader(path)
    if err != nil {
        return fmt.Errorf("invalid docx: %w", err)
    }
    defer r.Close()
    
    for _, f := range r.File {
        // 防止 zip bomb
        if f.UncompressedSize64 > 100*1024*1024 {
            return fmt.Errorf("file too large after decompression")
        }
        // 防止路径遍历
        if strings.Contains(f.Name, "..") {
            return fmt.Errorf("invalid path in zip: %s", f.Name)
        }
    }
    return nil
}
```

## 5. API 速率限制

### 5.1 全局限流

```go
func RateLimitMiddleware(store *redis.Client, limit rate.Limit, burst int) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := "ratelimit:" + c.ClientIP()
        
        // 滑动窗口计数器（Redis）
        count, _ := store.Incr(ctx, key).Result()
        if count == 1 {
            store.Expire(ctx, key, time.Minute)
        }
        
        if count > int64(burst) {
            c.Header("Retry-After", "60")
            c.AbortWithStatusJSON(429, gin.H{
                "error": gin.H{
                    "code":    "RATE_LIMITED",
                    "message": "Too many requests",
                },
            })
            return
        }
        
        c.Header("X-RateLimit-Limit", strconv.Itoa(burst))
        c.Header("X-RateLimit-Remaining", strconv.Itoa(burst-int(count)))
        c.Next()
    }
}
```

### 5.2 限流策略

| 端点类型 | 限制 | 说明 |
| :--- | :--- | :--- |
| Session 创建 | 10/min per IP | 防止滥用上传 |
| Round 启动 | 30/min per IP | LLM 调用成本控制 |
| 查询/导出 | 120/min per IP | 宽松的读取限制 |
| Webhook 配置 | 5/min per IP | 管理操作低频 |
| 健康检查 | 无限制 | K8s probe 需要 |

## 6. 审计日志

所有敏感操作记录到 `audit_log` 表：

```go
func (m *AuditMiddleware) Log(action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        
        m.db.Create(&AuditLog{
            SessionID: getSessionID(c),
            Action:    action,
            Actor:     getUserID(c),
            Detail: map[string]interface{}{
                "method":    c.Request.Method,
                "path":      c.Request.URL.Path,
                "status":    c.Writer.Status(),
                "duration":  time.Since(start).Milliseconds(),
                "client_ip": c.ClientIP(),
                "user_agent": c.Request.UserAgent(),
            },
        })
    }
}
```

| 事件 | 说明 |
| :--- | :--- |
| `session.created` | 上传文档创建会话 |
| `round.started` | 开始处理轮次 |
| `round.paused` | 暂停处理 |
| `round.resumed` | 恢复处理 |
| `round.completed` | 轮次完成 |
| `session.exported` | 下载输出文件 |
| `session.deleted` | 删除会话 |
| `config.updated` | 更新模型配置 |
| `webhook.created` | 创建 Webhook |
| `webhook.deleted` | 删除 Webhook |
| `provider.failover` | LLM Provider 切换 |

## 7. 数据加密

### 7.1 传输加密

- TLS 1.3 用于所有外部通信
- Ingress 终止 TLS，集群内部可选 mTLS

### 7.2 静态加密

- PostgreSQL：云厂商磁盘加密（AWS RDS 加密 / 阿里云 TDE）
- Redis：AOF/RDB 文件加密（云托管版自带）

### 7.3 字段级加密

LLM API Key 使用 KMS 加密后存储：

```go
type SecureConfig struct {
    APIKeyEncrypted string `json:"api_key_encrypted" gorm:"column:api_key_encrypted"`
}

func (c *SecureConfig) GetAPIKey(kms KMSClient) (string, error) {
    return kms.Decrypt(c.APIKeyEncrypted)
}

func (c *SecureConfig) SetAPIKey(kms KMSClient, plainKey string) error {
    encrypted, err := kms.Encrypt(plainKey)
    if err != nil {
        return err
    }
    c.APIKeyEncrypted = encrypted
    return nil
}
```

### 7.4 敏感数据处理

- API Key 在日志中自动脱敏：只显示前 8 位 + `***`
- Prompt 内容和文档原文**不写入日志**（DEBUG 级别除外，且仅限开发环境）
- Webhook Secret 创建后不可再读取（仅存 hash）
