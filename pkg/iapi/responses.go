package iapi

import "time"

// BaseResponse reflects internal-apis JSON response format.
type BaseResponse struct {
	Success bool        `json:"success"`
	Error   *string     `json:"error"`
	Data    interface{} `json:"data"`
}

type UserHasVerifiedEmailResponse struct {
	Success bool        `json:"success"`
	Error   interface{} `json:"error"`
	Data    struct {
		UserID           int  `json:"user_id"`
		HasVerifiedEmail bool `json:"has_verified_email"`
	} `json:"data"`
}

type CustomerListResponse struct {
	Success bool    `json:"success"`
	Error   *string `json:"error"`
	Data    []struct {
		ID               int       `json:"id"`
		TipperUserID     int       `json:"tipper_user_id"`
		CreatorUserID    int       `json:"creator_user_id"`
		AccountID        int       `json:"account_id"`
		ChannelName      string    `json:"channel_name"`
		ChannelClaimID   string    `json:"channel_claim_id"`
		TippedAmount     int       `json:"tipped_amount"`
		ReceivedAmount   int       `json:"received_amount"`
		Currency         string    `json:"currency"`
		TargetClaimID    string    `json:"target_claim_id"`
		Status           string    `json:"status"`
		PaymentIntentID  string    `json:"payment_intent_id"`
		PrivateTip       bool      `json:"private_tip"`
		CreatedAt        time.Time `json:"created_at"`
		UpdatedAt        time.Time `json:"updated_at"`
		Type             string    `json:"type"`
		ReferenceClaimID *string   `json:"reference_claim_id"`
		ValidThrough     time.Time `json:"valid_through"`
		SourceClaimID    string    `json:"source_claim_id"`
	} `json:"data"`
}
