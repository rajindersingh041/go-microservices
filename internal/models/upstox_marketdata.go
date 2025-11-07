package models

// ApiResponse, DataStruct, Ohlc... (No change)
type ApiResponse struct {
	Data    DataStruct `json:"data"`
	Success bool       `json:"success"`
}
type DataStruct struct {
	RequestID    string               `json:"request_id"`
	TimeInMillis int64                `json:"time_in_millis"`
	TokenData    map[string]TokenInfo `json:"token_data"`
}
type Ohlc struct {
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume int64   `json:"volume"`
}

// --- THIS IS THE UPDATED STRUCT ---
// TokenInfo now uses a pointer for Ohlc
type TokenInfo struct {
	Timestamp         string  `json:"timestamp"`
	LastTradeTime     string  `json:"lastTradeTime"`
	LastPrice         float64 `json:"lastPrice"`
	ClosePrice        float64 `json:"closePrice"`
	LastQuantity      int64   `json:"lastQuantity"`      // New
	BuyQuantity       float64 `json:"buyQuantity"`      // New
	SellQuantity      float64 `json:"sellQuantity"`     // New
	Volume            int64   `json:"volume"`           // New
	AveragePrice      float64 `json:"averagePrice"`     // New
	Oi                float64 `json:"oi"`               // New
	Poi               float64 `json:"poi"`              // New
	OiDayHigh         float64 `json:"oiDayHigh"`        // New
	OiDayLow          float64 `json:"oiDayLow"`         // New
	NetChange         float64 `json:"netChange"`
	LowerCircuitLimit float64 `json:"lowerCircuitLimit"` // New
	UpperCircuitLimit float64 `json:"upperCircuitLimit"` // New
	Yl                float64 `json:"yl"`                // New
	Yh                float64 `json:"yh"`                // New
	Ohlc              *Ohlc   `json:"ohlc"` // Still optional
}