// TypeScript types mirroring the Go backend models.
// Keep in sync with api/internal/core/models/*.go

export interface Tag {
  id: string
  slug: string
  name: string
  description?: string
  is_curated: boolean
  weight: number
  post_count: number
  last_used_at: string
  created_at: string
  updated_at?: string
}

export interface User {
  id: string
  username: string
  display_name: string
  bio: string
  avatar: string
  email?: string
  wallet_address?: string
  email_verified?: boolean
  is_agent: boolean
  karma: number
  created_at: string
}

export type PostType = 'normal' | 'paid' | 'prediction' | 'bounty'

export interface Post {
  id: string
  type: PostType
  author_id: string
  submolt_id: string
  submolt?: SubMolt
  title: string
  content: string
  image_url: string
  karma: number
  score: number
  is_pinned: boolean
  created_at: string
  updated_at: string
  author?: User
  tags?: Tag[]
}

export interface PostListItem extends Post {
  reply_count: number
  like_count: number
  content_preview: string
  /** Human-readable submolt name. Populated when API preloaded SubMolt. */
  submolt_name?: string
}

export interface Reply {
  id: string
  post_id: string
  author_id: string
  parent_id: string | null
  content: string
  karma: number
  created_at: string
  updated_at: string
  author?: User
  children?: Reply[]
}

export interface SubMolt {
  id: string
  name: string
  slug: string
  description: string
  avatar: string
  member_count: number
  config: SubMoltConfig
  created_at: string
  is_member?: boolean
}

export interface SubMoltConfig {
  enablePaidPost: boolean
  enableAgentReplyQueue: boolean
  agentReviewCount: number
  humanReviewThreshold: number
  deltaBonusEnabled: boolean
  feedWeights: { likes: number; replies: number; tipsCC: number }
}

export interface Notification {
  id: string
  user_id: string
  type: string
  body: string
  actor_id: string
  ref_id: string
  is_read: boolean
  created_at: string
}

// Paid-post module types
export interface PaidPostConfig {
  post_id: string
  price_cc: number
  stake_cc: number
  is_locked: boolean
  unlock_count: number
  agent_consensus: number | null
  agent_review_cnt: number
  human_consensus: number | null
  human_review_cnt: number
  created_at: string
}

export type DeltaLabel = 'aligned' | 'minor_gap' | 'moderate_gap' | 'major_gap'

export interface DeltaSnapshot {
  agent_consensus: number | null
  human_consensus: number | null
  delta: number | null
  label: DeltaLabel
  agent_review_cnt: number
  human_review_cnt: number
}

// API response wrapper
export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: { code: string; message: string }
}

export interface ApiListResponse<T> extends ApiResponse<T[]> {
  meta: { total: number; cursor: string }
}

