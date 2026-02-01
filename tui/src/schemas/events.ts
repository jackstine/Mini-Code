import { z } from "zod"

// Individual event schemas
const UserEventSchema = z.object({
  type: z.literal("user"),
  content: z.string(),
  timestamp: z.number()
})

const TextEventSchema = z.object({
  type: z.literal("text"),
  content: z.string(),
  timestamp: z.number()
})

const ToolCallEventSchema = z.object({
  type: z.literal("tool_call"),
  id: z.string(),
  name: z.string(),
  input: z.unknown(), // Accept any valid JSON type, not just objects
  timestamp: z.number()
})

const ToolResultEventSchema = z.object({
  type: z.literal("tool_result"),
  id: z.string(),
  result: z.string(),
  isError: z.boolean(),
  timestamp: z.number()
})

const ReasoningEventSchema = z.object({
  type: z.literal("reasoning"),
  content: z.string(),
  timestamp: z.number()
})

const StatusEventSchema = z.object({
  type: z.literal("status"),
  state: z.enum(["idle", "thinking", "running_tool", "error"]),
  message: z.string().optional(),
  timestamp: z.number().optional()
})

// Discriminated union for efficient parsing
export const EventSchema = z.discriminatedUnion("type", [
  UserEventSchema,
  TextEventSchema,
  ToolCallEventSchema,
  ToolResultEventSchema,
  ReasoningEventSchema,
  StatusEventSchema,
])

// Type inference
export type Event = z.infer<typeof EventSchema>
export type UserEvent = z.infer<typeof UserEventSchema>
export type TextEvent = z.infer<typeof TextEventSchema>
export type ToolCallEvent = z.infer<typeof ToolCallEventSchema>
export type ToolResultEvent = z.infer<typeof ToolResultEventSchema>
export type ReasoningEvent = z.infer<typeof ReasoningEventSchema>
export type StatusEvent = z.infer<typeof StatusEventSchema>
