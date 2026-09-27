package scanner

import "fmt"

// companyNames maps common Bluetooth SIG company IDs to readable vendor names.
var companyNames = map[int]string{
	2:   "Intel",
	6:   "Microsoft",
	7:   "Ericsson",
	10:  "Intel",
	13:  "Texas Instruments",
	37:  "Hewlett-Packard",
	47:  "Asustek",
	49:  "Broadcom",
	65:  "Garmin",
	67:  "Logitech",
	76:  "Apple",
	78:  "HTC",
	87:  "Sony",
	89:  "Nordic Semiconductor",
	93:  "Sierra Wireless",
	94:  "Google",
	100: "Fitbit",
	117: "Samsung",
	129: "Roku",
	133: "Google",
	196: "Fitbit",
	203: "Fitbit",
	205: "Amazon",
	224: "Google",
	247: "Meta",
	266: "Huawei",
	271: "Xiaomi",
	305: "GoPro",
	510: "Google",
	526: "Amazon",
	639: "Microsoft",
	650: "Apple",
	651: "OnePlus",
}

// CompanyName maps a Bluetooth SIG company ID to a vendor name, or hex if unknown.
func CompanyName(id int) string {
	if id == 0 {
		return ""
	}
	if n, ok := companyNames[id]; ok {
		return n
	}
	return fmt.Sprintf("0x%04X", id)
}
