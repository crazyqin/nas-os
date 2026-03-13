package webdav

import "encoding/xml"

// PropFind PROPFIND 请求
type PropFind struct {
	XMLName  xml.Name `xml:"propfind"`
	AllProp  *struct{} `xml:"allprop"`
	PropName *struct{} `xml:"propname"`
	Prop     *PropRequest `xml:"prop"`
}

// PropRequest 请求的属性
type PropRequest struct {
	XMLName          xml.Name `xml:"prop"`
	Displayname      *struct{} `xml:"displayname"`
	GetContentType   *struct{} `xml:"getcontenttype"`
	GetContentLength *struct{} `xml:"getcontentlength"`
	GetLastModified  *struct{} `xml:"getlastmodified"`
	GetETag          *struct{} `xml:"getetag"`
	ResourceType     *struct{} `xml:"resourcetype"`
}

// Multistatus 多状态响应
type Multistatus struct {
	XMLName   xml.Name          `xml:"DAV:multistatus"`
	Responses []PropfindResponse `xml:"response"`
}

// PropfindResponse PROPFIND 响应
type PropfindResponse struct {
	XMLName  xml.Name   `xml:"response"`
	Href     string     `xml:"href"`
	PropStat []PropStat `xml:"propstat"`
}

// PropStat 属性状态
type PropStat struct {
	XMLName xml.Name `xml:"propstat"`
	Prop    Prop     `xml:"prop"`
	Status  string   `xml:"status"`
}

// Prop 属性
type Prop struct {
	XMLName          xml.Name      `xml:"prop"`
	Displayname      string        `xml:"displayname,omitempty"`
	GetContentType   string        `xml:"getcontenttype,omitempty"`
	GetContentLength int64         `xml:"getcontentlength,omitempty"`
	GetLastModified  string        `xml:"getlastmodified,omitempty"`
	GetETag          string        `xml:"getetag,omitempty"`
	ResourceType     *ResourceType `xml:"resourcetype,omitempty"`
	LockDiscovery    *LockDiscovery `xml:"lockdiscovery,omitempty"`
}

// ResourceType 资源类型
type ResourceType struct {
	XMLName    xml.Name `xml:"resourcetype"`
	Collection *struct{} `xml:"collection,omitempty"`
}