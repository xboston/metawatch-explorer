package main

import (
	"fmt"
)

func prettyPrintJSON(b interface{}) ([]byte, error) {
	return json.MarshalIndent(&b, "", "    ")
}

func byteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func hashtrim(hash string) string {

	// address
	if len(hash) == 52 || hash == "InitialWalletTransaction" {
		go updateAddress(hash)

		if n, ok := nodeNames[hash]; ok {

			name := n
			lenName := len(name)
			if lenName > 20 {
				name = name[0:15] + "…" + name[lenName-3:lenName]
			}

			return name
		}

		return hash[0:12] + "…" + hash[49:52]
	}

	// trx
	if len(hash) == 64 {
		return hash[0:12] + "…" + hash[60:64]
	}

	return hash
}
