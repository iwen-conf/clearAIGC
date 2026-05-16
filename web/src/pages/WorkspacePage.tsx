import { Alert, Button, Col, Row, Space, Spin, Tabs, Typography } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import { HistoryOutlined } from "@ant-design/icons";
import { Link, useNavigate } from "react-router-dom";
import { useSession } from "@/hooks/use-session";
import { UploadCard } from "@/components/workspace/upload-card";
import { SessionHeader } from "@/components/workspace/session-header";
import { ProgressInspector } from "@/components/workspace/progress-inspector";
import { ActivityTimeline } from "@/components/workspace/activity-timeline";
import { PreviewPane } from "@/components/workspace/preview-pane";
import { CardReviewPanel } from "@/components/workspace/card-review-panel";

const { Text } = Typography;

export default function WorkspacePage() {
  const state = useSession();
  const navigate = useNavigate();

  if (state.booting) {
    return (
      <PageContainer>
        <div style={{ display: "flex", justifyContent: "center", padding: 64 }}>
          <Spin tip="正在恢复会话…" />
        </div>
      </PageContainer>
    );
  }

  if (!state.session) {
    return (
      <PageContainer
        title="开始一次新的润色"
        subTitle={
          <Text type="secondary">
            上传稿件,选择模式,系统将逐段润色并实时反馈进度
          </Text>
        }
        extra={[
          <Button
            key="history"
            icon={<HistoryOutlined />}
            onClick={() => navigate("/history")}
          >
            历史记录
          </Button>,
        ]}
      >
        <UploadCard
          busy={state.busy}
          error={state.error}
          message={state.message}
          onSubmit={state.start}
        />
        <div style={{ textAlign: "center", marginTop: 16 }}>
          <Text type="secondary">
            或从
            <Link to="/history" style={{ margin: "0 4px" }}>
              历史记录
            </Link>
            中继续之前的文档 →
          </Text>
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer
      header={{ title: "润色工作台", ghost: true }}
      extra={[
        <Button
          key="history"
          icon={<HistoryOutlined />}
          onClick={() => navigate("/history")}
        >
          历史记录
        </Button>,
      ]}
    >
      <SessionHeader
        session={state.session}
        busy={state.busy}
        completedPassCount={state.completedPassCount}
        totalPasses={state.totalPasses}
        nextRoundNumber={state.nextRoundNumber}
        canStartNextRound={state.canStartNextRound}
        latestCompletedRoundNumber={state.latestCompletedRoundNumber}
        onPause={state.pause}
        onResume={state.resume}
        onStartNext={state.startNextRound}
        onRefresh={state.refresh}
        onReset={state.reset}
      />

      {(state.error || state.message) && (
        <Alert
          style={{ marginBottom: 16 }}
          showIcon
          type={state.error ? "error" : "info"}
          message={state.error ? "出现问题" : "提示"}
          description={state.error ?? state.message}
        />
      )}

      <Row gutter={16}>
        <Col xs={24} lg={16}>
          <Tabs
            defaultActiveKey="preview"
            items={[
              {
                key: "preview",
                label: "文稿预览",
                children: <PreviewPane preview={state.preview} />,
              },
              {
                key: "sections",
                label: "卡片评审",
                children: (
                  <CardReviewPanel
                    comparison={state.comparison}
                    sessionId={state.session?.id ?? null}
                    roundNumber={state.latestCompletedRoundNumber}
                    cardBusyId={state.cardBusyId}
                    applyingAll={state.applyingAll}
                    onAccept={state.acceptCard}
                    onReject={state.rejectCard}
                    onApplyAll={state.applyAll}
                  />
                ),
              },
            ]}
          />
        </Col>
        <Col xs={24} lg={8}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <ProgressInspector
              session={state.session}
              progress={state.progress}
              completedPassCount={state.completedPassCount}
              totalPasses={state.totalPasses}
              nextRoundNumber={state.nextRoundNumber}
            />
            <ActivityTimeline entries={state.timeline} />
          </Space>
        </Col>
      </Row>
    </PageContainer>
  );
}
