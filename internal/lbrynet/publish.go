package lbrynet

type PublishParams struct {
	Name              string        `json:"name"`
	Title             string        `json:"title,omitempty"`
	Description       string        `json:"description,omitempty"`
	AccountID         string        `json:"account_id"`
	Bid               string        `json:"bid,omitempty"`
	FeeCurrency       string        `json:"fee_currency,omitempty"`
	FeeAmount         string        `json:"fee_amount,omitempty"`
	FeeAddress        string        `json:"fee_address,omitempty"`
	FilePath          string        `json:"file_path"`
	Author            string        `json:"author,omitempty"`
	ThumbnailURL      string        `json:"thumbnail_url,omitempty"`
	LicenseURL        string        `json:"license_url,omitempty"`
	License           string        `json:"license,omitempty"`
	ReleaseTime       int32         `json:"release_time,omitempty"`
	Width             int32         `json:"width,omitempty"`
	Height            int32         `json:"height,omitempty"`
	Duration          int32         `json:"duration,omitempty"`
	ChannelID         string        `json:"channel_id,omitempty"`
	ChannelName       string        `json:"channel_name,omitempty"`
	Tags              []interface{} `json:"tags,omitempty"`
	Languages         []interface{} `json:"languages,omitempty"`
	Locations         []interface{} `json:"locations,omitempty"`
	ChannelAccountID  []interface{} `json:"channel_account_id,omitempty"`
	FundingAccountIDs []interface{} `json:"funding_account_ids,omitempty"`
	ClaimAddress      string        `json:"claim_address,omitempty"`
	Preview           bool          `json:"preview,omitempty"`
	Blocking          bool          `json:"blocking,omitempty"`
}

type PublishResponse struct {
	Txid   string `json:"txid"`
	Height string `json:"height"`
	Inputs []struct {
		Txid                    string `json:"txid"`
		Nout                    string `json:"nout"`
		Height                  string `json:"height"`
		Amount                  string `json:"amount"`
		Address                 string `json:"address"`
		Confirmations           string `json:"confirmations"`
		IsChange                string `json:"is_change"`
		IsMine                  string `json:"is_mine"`
		Type                    string `json:"type"`
		Name                    string `json:"name"`
		ClaimID                 string `json:"claim_id"`
		ClaimOp                 string `json:"claim_op"`
		Value                   string `json:"value"`
		ValueType               string `json:"value_type"`
		Protobuf                string `json:"protobuf"`
		PermanentURL            string `json:"permanent_url"`
		SigningChannel          string `json:"signing_channel"`
		IsChannelSignatureValid string `json:"is_channel_signature_valid"`
	} `json:"inputs"`
	Outputs []struct {
		Txid                    string `json:"txid"`
		Nout                    string `json:"nout"`
		Height                  string `json:"height"`
		Amount                  string `json:"amount"`
		Address                 string `json:"address"`
		Confirmations           string `json:"confirmations"`
		IsChange                string `json:"is_change"`
		IsMine                  string `json:"is_mine"`
		Type                    string `json:"type"`
		Name                    string `json:"name"`
		ClaimID                 string `json:"claim_id"`
		ClaimOp                 string `json:"claim_op"`
		Value                   string `json:"value"`
		ValueType               string `json:"value_type"`
		Protobuf                string `json:"protobuf"`
		PermanentURL            string `json:"permanent_url"`
		SigningChannel          string `json:"signing_channel"`
		IsChannelSignatureValid string `json:"is_channel_signature_valid"`
	} `json:"outputs"`
	TotalInput  string `json:"total_input"`
	TotalOutput string `json:"total_output"`
	TotalFee    string `json:"total_fee"`
	Hex         string `json:"hex"`
}
