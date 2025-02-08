package config

import "time"

var LimitCookies = make(map[string]time.Time)

func CheckCookieLimit(cookie string) bool {
	if c, ok := LimitCookies[cookie]; ok {
		if c.Add(FreeLimitDisableCookieDuration).Before(time.Now()) {
			return false
		}
		return true
	}
	return false
}

func CookieLimit(cookie string) {
	LimitCookies[cookie] = time.Now()
}

func (cm *CookieManager) GetNoLimitCookie() {
	if len(LimitCookies) == 0 {
		return
	}
	for _, cookie := range cm.Cookies {
		if CheckCookieLimit(cookie) {
			_ = cm.RemoveCookie(cookie)
		}
	}
}
