import { Descriptions, Progress, Space, Typography } from "antd";
import { ThunderboltOutlined } from "@ant-design/icons";
import { ProCard } from "@ant-design/pro-components";
import type { ProgressPayload, Session } from "@/types";

const { Text } = Typography;

interface ProgressInspectorProps {
  session: Session;
  progress: ProgressPayload | null;
  completedPassCount: number;
  totalPasses: number;
  nextRoundNumber: number | null;
}

export function ProgressInspector({
  session,
  progress,
  completedPassCount,
  totalPasses,
  nextRoundNumber,
}: ProgressInspectorProps) {
  const percent = Math.round(
    progress?.percent ?? (session.status === "completed" ? 100 : 0),
  );
  const currentRound =
    progress?.round ??
    (session.status === "processing" ? nextRoundNumber : completedPassCount);

  return (
    <ProCard
      title={
        <Space size={8}>
          <ThunderboltOutlined />
          处理流水线
        </Space>
      }
      headerBordered
      size="small"
    >
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <div>
          <Space style={{ width: "100%", justifyContent: "space-between" }}>
            <Text type="secondary">
              第 {currentRound || "–"} / {totalPasses} 轮
            </Text>
            <Text code>{percent}%</Text>
          </Space>
          <Progress
            percent={percent}
            showInfo={false}
            status={session.status === "processing" ? "active" : undefined}
            style={{ marginTop: 4 }}
          />
        </div>

        <Descriptions size="small" column={2} colon={false}>
          <Descriptions.Item label="段落块">
            <Text code>
              {progress
                ? `${progress.completedChunks}/${progress.totalChunks}`
                : "–/–"}
            </Text>
          </Descriptions.Item>
          <Descriptions.Item label="阶段">
            {progress?.phase ?? "空闲"}
          </Descriptions.Item>
          <Descriptions.Item label="模型">
            {progress?.providerUsed || "—"}
          </Descriptions.Item>
          <Descriptions.Item label="已完成轮次">
            <Text code>
              {completedPassCount}/{totalPasses}
            </Text>
          </Descriptions.Item>
        </Descriptions>
      </Space>
    </ProCard>
  );
}
