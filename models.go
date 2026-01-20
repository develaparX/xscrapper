package main

// TwitterResponse represents the response from Twitter messages API
type TwitterResponse struct {
	Code    int              `json:"code"`
	Reason  string           `json:"reason"`
	Message string           `json:"message"`
	Data    []TwitterMessage `json:"data"`
}

// TwitterMessage represents a single Twitter message
type TwitterMessage struct {
	ID            string         `json:"id"`
	Platform      int            `json:"platform"`
	TweetType     string         `json:"tw_type"`
	SubTweetType  string         `json:"sub_tw_type,omitempty"`
	TweetID       string         `json:"tweet_id"`
	Complete      int            `json:"complete"`
	Timestamp     string         `json:"tw_timestamp"`
	User          TwitterUser    `json:"user"`
	UserTags      []string       `json:"user_tags"`
	Content       TweetContent   `json:"content,omitempty"`
	Action        *TwitterAction `json:"action,omitempty"`
	Profile       *ProfileChange `json:"profile,omitempty"`
	SourceID      string         `json:"source_id,omitempty"`
	SourceUser    *TwitterUser   `json:"source_user,omitempty"`
	SourceContent *TweetContent  `json:"source_content,omitempty"`
	TokenType     string         `json:"tw_token_type,omitempty"`
	Token         *TokenInfo     `json:"token,omitempty"`
}

// TwitterUser represents a Twitter user
type TwitterUser struct {
	ScreenName string `json:"screen_name"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
	Followers  int    `json:"followers,omitempty"`
}

// TweetContent represents tweet content
type TweetContent struct {
	Text   string      `json:"text"`
	URLs   []TweetURL  `json:"url,omitempty"`
	Media  []TweetMedia `json:"media,omitempty"`
}

// TweetURL represents a URL in a tweet
type TweetURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// TweetMedia represents media (images, videos) in a tweet
type TweetMedia struct {
	Type string `json:"type"` // "image", "video", "thumbnail"
	URL  string `json:"url"`  // Direct URL to media
}

// TwitterAction represents actions like follow, like, etc.
type TwitterAction struct {
	Follow *FollowAction `json:"follow,omitempty"`
}

// FollowAction represents a follow action
type FollowAction struct {
	Following *FollowUser `json:"following,omitempty"`
	User      *TwitterUser `json:"user,omitempty"`
}

// FollowUser represents a user being followed (with additional info)
type FollowUser struct {
	ScreenName    string `json:"screen_name"`
	Name          string `json:"name"`
	Avatar        string `json:"avatar"`
	Followers     int    `json:"followers,omitempty"`
	KeyFollowers  int    `json:"key_followers,omitempty"`
	JoinedAt      int64  `json:"joined_at,omitempty"`
	Description   string `json:"description,omitempty"`
}

// ProfileChange represents profile changes like handle updates, name changes, and bio changes
type ProfileChange struct {
	BeforeHandle    string `json:"before_handle,omitempty"`
	AfterHandle     string `json:"after_handle,omitempty"`
	BeforeName      string `json:"before_name,omitempty"`
	AfterName       string `json:"after_name,omitempty"`
	BeforeDescription string `json:"before_description,omitempty"`
	AfterDescription  string `json:"after_description,omitempty"`
	Description     string `json:"description,omitempty"` // Current description for description type
}

// TokenInfo represents token information
type TokenInfo struct {
	Chain     string `json:"chain"`
	Symbol    string `json:"symbol"`
	Address   string `json:"ca"`
	Icon      string `json:"icon"`
	Price     string `json:"price"`
	Price24h  string `json:"price_24h,omitempty"`
	MarketCap string `json:"mcap"`
}

// WalletsResponse represents the response from following wallets API
type WalletsResponse struct {
	Code    int         `json:"code"`
	Reason  string      `json:"reason"`
	Message string      `json:"message"`
	Data    WalletsData `json:"data"`
}

// WalletsData contains wallet list and pagination info
type WalletsData struct {
	List    []Wallet `json:"list"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	Limit   int      `json:"limit"`
	HasMore bool     `json:"has_more"`
}

// Wallet represents a wallet being followed
type Wallet struct {
	WalletAddress string   `json:"wallet_address"`
	WalletTag     string   `json:"wallet_tag,omitempty"`
	Chain         string   `json:"chain"`
	Balance       string   `json:"balance,omitempty"`
	PNL           string   `json:"pnl,omitempty"`
	WinRate       float64  `json:"win_rate,omitempty"`
	TotalTrades   int      `json:"total_trades,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	TwitterHandle string   `json:"twitter_handle,omitempty"`
	ENS           string   `json:"ens,omitempty"`
}
