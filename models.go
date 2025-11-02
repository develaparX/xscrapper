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
	ID            string        `json:"id"`
	Platform      int           `json:"platform"`
	TweetType     string        `json:"tw_type"`
	TweetID       string        `json:"tweet_id"`
	Complete      int           `json:"complete"`
	Timestamp     string        `json:"tw_timestamp"`
	User          TwitterUser   `json:"user"`
	UserTags      []string      `json:"user_tags"`
	Content       TweetContent  `json:"content"`
	SourceID      string        `json:"source_id,omitempty"`
	SourceUser    *TwitterUser  `json:"source_user,omitempty"`
	SourceContent *TweetContent `json:"source_content,omitempty"`
	TokenType     string        `json:"tw_token_type,omitempty"`
	Token         *TokenInfo    `json:"token,omitempty"`
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
	Text string     `json:"text"`
	URLs []TweetURL `json:"url,omitempty"`
}

// TweetURL represents a URL in a tweet
type TweetURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
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
