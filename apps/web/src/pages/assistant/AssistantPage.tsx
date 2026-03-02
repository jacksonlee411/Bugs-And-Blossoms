import SmartToyIcon from "@mui/icons-material/SmartToy"
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  TextField,
  Typography
} from "@mui/material"
import { useCallback, useEffect, useMemo, useState } from "react"
import {
  commitAssistantTurn,
  confirmAssistantTurn,
  createAssistantConversation,
  createAssistantTurn,
  type AssistantConversation,
  type AssistantTurn
} from "../../api/assistant"

const samplePrompt =
  "在鲜花组织之下，新建一个名为运营部的部门，成立日期是2026年1月1日。通过AI对话，调用相关能力完成部门的创建任务。"

const assistantStatePlaceholder = "-"

function latestTurn(conversation: AssistantConversation | null): AssistantTurn | null {
  if (!conversation || conversation.turns.length === 0) {
    return null
  }
  const turn = conversation.turns[conversation.turns.length - 1]
  return turn ?? null
}

export function AssistantPage() {
  const [conversation, setConversation] = useState<AssistantConversation | null>(null)
  const [input, setInput] = useState(samplePrompt)
  const [candidateID, setCandidateID] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  const turn = useMemo(() => latestTurn(conversation), [conversation])

  useEffect(() => {
    let active = true
    setLoading(true)
    createAssistantConversation()
      .then((result) => {
        if (!active) {
          return
        }
        setConversation(result)
      })
      .catch((err: { message?: string }) => {
        if (!active) {
          return
        }
        setError(err.message ?? "创建会话失败")
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [])

  const handleGenerate = useCallback(async () => {
    if (!conversation) {
      return
    }
    const text = input.trim()
    if (!text) {
      setError("请输入对话内容")
      return
    }
    setError("")
    setLoading(true)
    try {
      const next = await createAssistantTurn(conversation.conversation_id, text)
      setConversation(next)
      const nextTurn = latestTurn(next)
      if (nextTurn?.resolved_candidate_id) {
        setCandidateID(nextTurn.resolved_candidate_id)
      } else {
        setCandidateID("")
      }
    } catch (err) {
      const message = (err as { message?: string }).message ?? "生成计划失败"
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [conversation, input])

  const handleConfirm = useCallback(async () => {
    if (!conversation || !turn) {
      return
    }
    setError("")
    setLoading(true)
    try {
      const next = await confirmAssistantTurn(conversation.conversation_id, turn.turn_id, candidateID || undefined)
      setConversation(next)
    } catch (err) {
      setError((err as { message?: string }).message ?? "确认失败")
    } finally {
      setLoading(false)
    }
  }, [candidateID, conversation, turn])

  const handleCommit = useCallback(async () => {
    if (!conversation || !turn) {
      return
    }
    setError("")
    setLoading(true)
    try {
      const next = await commitAssistantTurn(conversation.conversation_id, turn.turn_id)
      setConversation(next)
    } catch (err) {
      setError((err as { message?: string }).message ?? "提交失败")
    } finally {
      setLoading(false)
    }
  }, [conversation, turn])

  return (
    <Stack spacing={2}>
      <Stack alignItems="center" direction="row" spacing={1}>
        <SmartToyIcon color="primary" />
        <Typography variant="h5">AI 助手</Typography>
      </Stack>
      <Typography color="text.secondary" variant="body2">
        左侧为 LibreChat 聊天展示层，右侧为本系统事务控制面板（Confirm / Commit 仍走后端 One Door）。
      </Typography>
      {error ? <Alert severity="error">{error}</Alert> : null}
      <Box sx={{ display: "flex", gap: 2, flexDirection: { xs: "column", md: "row" } }}>
        <Box sx={{ flex: 7 }}>
          <Card sx={{ height: "100%" }}>
            <CardContent>
              <Typography gutterBottom variant="subtitle1">
                聊天与计划展示层（LibreChat）
              </Typography>
              <Box
                component="iframe"
                src="/assistant-ui/"
                title="LibreChat"
                sx={{ width: "100%", height: 580, border: "1px solid", borderColor: "divider", borderRadius: 1 }}
              />
            </CardContent>
          </Card>
        </Box>
        <Box sx={{ flex: 5 }}>
          <Card>
            <CardContent>
              <Stack spacing={2}>
                <Typography variant="subtitle1">事务控制面板</Typography>
                <TextField
                  label="输入需求"
                  minRows={4}
                  multiline
                  onChange={(event) => setInput(event.target.value)}
                  value={input}
                />
                <Button disabled={loading || !conversation} onClick={() => void handleGenerate()} variant="contained">
                  生成计划
                </Button>
                <Divider />
                <Typography variant="body2">conversation_id: {conversation?.conversation_id ?? "-"}</Typography>
                <Typography variant="body2">turn_id: {turn?.turn_id ?? "-"}</Typography>
                <Typography variant="body2">request_id: {turn?.request_id ?? "-"}</Typography>
                <Typography variant="body2">trace_id: {turn?.trace_id ?? "-"}</Typography>
                <Stack alignItems="center" direction="row" spacing={1}>
                  <Typography variant="body2">状态：</Typography>
                  <Chip label={turn?.state ?? assistantStatePlaceholder} size="small" />
                  <Chip
                    color={turn?.risk_tier === "high" ? "warning" : "default"}
                    label={`risk=${turn?.risk_tier ?? "-"}`}
                    size="small"
                  />
                </Stack>
                {turn?.plan ? (
                  <Alert severity="info">
                    <strong>{turn.plan.title}</strong>
                    <br />
                    {turn.plan.summary}
                  </Alert>
                ) : null}
                {turn?.candidates?.length ? (
                  <Stack spacing={1}>
                    <Typography variant="subtitle2">父组织候选</Typography>
                    <RadioGroup onChange={(_, value) => setCandidateID(value)} value={candidateID}>
                      {turn.candidates.map((candidate) => (
                        <FormControlLabel
                          control={<Radio />}
                          key={candidate.candidate_id}
                          label={`${candidate.name} / ${candidate.candidate_code} / ${candidate.path} / ${candidate.as_of}`}
                          value={candidate.candidate_id}
                        />
                      ))}
                    </RadioGroup>
                  </Stack>
                ) : null}
                <Stack direction="row" spacing={1}>
                  <Button disabled={loading || !turn} onClick={() => void handleConfirm()} variant="outlined">
                    Confirm
                  </Button>
                  <Button disabled={loading || !turn} onClick={() => void handleCommit()} variant="contained">
                    Commit
                  </Button>
                </Stack>
                {turn?.commit_result ? (
                  <Alert severity="success">
                    已提交：org_code={turn.commit_result.org_code} / parent={turn.commit_result.parent_org_code} /
                    effective_date={turn.commit_result.effective_date}
                  </Alert>
                ) : null}
              </Stack>
            </CardContent>
          </Card>
        </Box>
      </Box>
    </Stack>
  )
}
