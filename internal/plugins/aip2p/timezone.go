package newsplugin

import "time"

func defaultDisplayLocation() *time.Location {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return loc
	}
	return time.FixedZone("UTC+8", 8*60*60)
}

func defaultDisplayTime(t time.Time) time.Time {
	return t.In(defaultDisplayLocation())
}

func defaultDisplayDate(t time.Time) string {
	return defaultDisplayTime(t).Format("2006-01-02")
}

