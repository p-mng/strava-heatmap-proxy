package main

type SessionCookieJar struct {
	Cookies []Cookie `json:"cookies"`
}

type Cookie struct {
	Host             string           `json:"host"`
	Value            string           `json:"value"`
	Path             string           `json:"path"`
	Name             string           `json:"name"`
	Secure           bool             `json:"secure,omitempty"`
	Httponly         bool             `json:"httponly,omitempty"`
	OriginAttributes OriginAttributes `json:"originAttributes"`
	SameSite         int              `json:"sameSite,omitempty"`
	SchemeMap        int              `json:"schemeMap"`
	IsPartitioned    bool             `json:"isPartitioned,omitempty"`
}

type OriginAttributes struct {
	FirstPartyDomain          string `json:"firstPartyDomain"`
	GeckoViewSessionContextID string `json:"geckoViewSessionContextId"`
	PartitionKey              string `json:"partitionKey"`
	PrivateBrowsingID         int    `json:"privateBrowsingId"`
	UserContextID             int    `json:"userContextId"`
}
