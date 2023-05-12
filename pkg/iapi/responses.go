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

type UserMeResponse struct {
	Success bool        `json:"success"`
	Error   interface{} `json:"error"`
	Data    struct {
		ID                  int      `json:"id"`
		Language            string   `json:"language"`
		GivenName           *string  `json:"given_name"`
		FamilyName          *string  `json:"family_name"`
		CreatedAt           string   `json:"created_at"`
		UpdatedAt           string   `json:"updated_at"`
		InvitedByID         int      `json:"invited_by_id"`
		InvitedAt           string   `json:"invited_at"`
		InvitesRemaining    int      `json:"invites_remaining"`
		InviteRewardClaimed bool     `json:"invite_reward_claimed"`
		IsEmailEnabled      bool     `json:"is_email_enabled"`
		PublishID           int      `json:"publish_id"`
		Country             string   `json:"country"`
		IsOdyseeUser        bool     `json:"is_odysee_user"`
		Location            *string  `json:"location"`
		PrimaryEmail        string   `json:"primary_email"`
		PasswordSet         bool     `json:"password_set"`
		LatestClaimedEmail  *string  `json:"latest_claimed_email"`
		HasVerifiedEmail    bool     `json:"has_verified_email"`
		IsIdentityVerified  bool     `json:"is_identity_verified"`
		IsRewardApproved    bool     `json:"is_reward_approved"`
		Groups              []string `json:"groups"`
		DeviceTypes         []string `json:"device_types"`
		GlobalMod           bool     `json:"global_mod"`
		ExperimentalUI      bool     `json:"experimental_ui"`
		InternalFeature     bool     `json:"internal_feature"`
		OdyseeMember        bool     `json:"odysee_member"`
		PendingDeletion     bool     `json:"pending_deletion"`
	} `json:"data"`
}

type MembershipPerkCheck struct {
	Success bool        `json:"success"`
	Error   interface{} `json:"error"`
	Data    struct {
		HasAccess bool `json:"has_access"`
	} `json:"data"`
}
