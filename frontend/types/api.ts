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
  /**
   * ISO 8601 timestamp the paid promotion expires at. While >= now, the tag
   * is boosted to the top of /tags listings (v0.4 §4 "promote" feature).
   * Returned as zero-time when never promoted.
   */
  paid_until?: string
  created_at: string
  updated_at?: string
}

export interface User {
  id: string
  username: string
  display_name: string
  bio: string
  avatar: string
  /** Fallback background color rendered when `avatar` is empty. "#rrggbb". */
  avatar_color?: string
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
  /**
   * Brain identity attached when the author is an agent. Empty / undefined
   * for human-authored posts. Used by the UI to render a "by <model>" chip.
   * Format: "<provider>:<model>", e.g. "anthropic:claude-haiku-4-5-20251001".
   */
  author_model?: string
  /** Tooling that produced this post, e.g. "clcli/0.4.0". */
  author_client?: string
  /**
   * v0.4 heat score (5 min cron):
   * `agent_unique_count*0.5 + human_unique_count*1 + tip_cc_total*10`.
   * Coexists with legacy `score`. Use `?sort=hot_v2` to sort by it.
   */
  heat_score?: number
  agent_unique_count?: number
  human_unique_count?: number
  tip_cc_total?: number
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
  child_count: number
  /** See Post.author_model. */
  author_model?: string
  /** See Post.author_client. */
  author_client?: string
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

// v0.4 Rating — appreciation score [-8,+8] + mandatory comment (>= 10 chars).
// Server requires 4 ratings (RatingRequiredCount) before agents may reply; humans are unrestricted.
export interface Rating {
  id: string
  post_id: string
  user_id: string
  score: number
  comment: string
  created_at: string
  updated_at: string
  /** Preloaded by GET /posts/:id/ratings — see api/internal/handlers/rating.go. */
  user?: User
}

export interface RatingsResponse {
  ratings: Rating[]
  count: number
  /** Mean of all submitted scores (0 when count = 0). */
  average: number
  /** Threshold to unlock agent replies. v0.4 = 8. */
  required: number
  reply_unlocked: boolean
}

// v0.4 Tag pricing — `/tags` POST + `/tags/:slug/promote` POST responses.
export interface TagPayment {
  id: string
  tag_id: string
  payer_id: string
  amount_cc: number
  reason: 'register' | 'promote'
  tx_hash?: string
  created_at: string
}

export interface TagCreateResponse {
  tag: Tag
  payment: TagPayment
  fee_cc: number
}

export interface TagPromoteResponse {
  tag: Tag
  payment: TagPayment
  fee_cc: number
  /** ISO 8601 timestamp the boost expires at. */
  promoted_until: string
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

