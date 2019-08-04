package main

import (
	"time"
)

type Address struct {
	Address         string `json:"address"`
	IsNode          bool   `json:"is_node" db:"is_node"`
	Name            string `json:"name"`
	Amount          int64  `json:"amount"`
	Frozen          int64  `json:"frozen"`
	Forging         int64  `json:"forging"`
	Delegated       int64  `json:"delegated"`
	Undelegated     int64  `json:"undelegated"`
	DelegatedAmount int64  `json:"delegated_amount"  db:"delegated_amount"`
	TxCount         int64  `json:"tx_count" db:"tx_count"`
	TxIn            int64  `json:"tx_in" db:"tx_in"`
	TxOut           int64  `json:"tx_out" db:"tx_out"`
	BlockNumber     int64  `json:"block_number" db:"block_number"`
	// Meta        string
	CreatedAt *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt *time.Time `json:"updated_at" db:"updated_at"`
}

type Node struct {
	Address  string `json:"address"`
	NodeType string `json:"type" db:"node_type"`

	Name         string  `json:"name"`
	IP           string  `json:"ip"`
	Latitude     float32 `json:"latitude"`
	Longitude    float32 `json:"longitude"`
	CountryShort string  `json:"country_short" db:"country_short"`
	CountryLong  string  `json:"country_long" db:"country_long"`
	Region       string  `json:"region"`
	City         string  `json:"city"`

	Geo    string `json:"geo" db:"mg_geo"`
	Status string `json:"status" db:"mg_status"`
	ROI    string `json:"roi" db:"mg_roi"`
	QPS    string `json:"qps" db:"mg_qps"`
	RPS    string `json:"rps" db:"mg_rps"`
	Trust  string `json:"trust" db:"mg_trust"`

	IsOnline bool `json:"is_online" db:"is_online"`

	LastUpdated *time.Time `json:"last_updated" db:"last_updated"`
	LastChecked *time.Time `json:"last_checked" db:"last_checked"`
	CreatedAt   *time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at" db:"updated_at"`
}
