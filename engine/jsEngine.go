package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PuerkitoBio/goquery"
	"github.com/dop251/goja"
	"io"
	"net/http"
	"net/url"
	"sync"
)

type Jar struct {
	lk      sync.Mutex
	cookies map[string][]*http.Cookie
}

func NewJar() *Jar {
	jar := new(Jar)
	jar.cookies = make(map[string][]*http.Cookie)
	return jar
}

// SetCookies handles the receipt of the cookies in a reply for the
// given URL.  It may or may not choose to save the cookies, depending
// on the jar's policy and implementation.
func (jar *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	jar.lk.Lock()
	jar.cookies[u.Host] = cookies
	jar.lk.Unlock()
}

// Cookies returns the cookies to send in a request for the given URL.
// It is up to the implementation to honor the standard cookie use
// restrictions such as in RFC 6265.
func (jar *Jar) Cookies(u *url.URL) []*http.Cookie {
	return jar.cookies[u.Host]
}

var jar = NewJar()

type Response struct {
	Headers  map[string]string `json:"headers"`
	BodyText string            `json:"bodyText"`
}

func (r *Response) Marshal() ([]byte, error) {
	return json.MarshalIndent(r, "", " ")
}

type RequestOptions struct {
	Method  string            `json:"method"`
	Url     string            `json:"url"`
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
	Cookies map[string]string `json:"cookies"`
}

func (r *RequestOptions) Unmarshal(data string) error {
	return json.Unmarshal([]byte(data), r)
}

func RequestWithRuntime(r *goja.Runtime) func(this goja.Value, args ...goja.Value) (goja.Value, error) {
	return func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		params := this.String()
		options := &RequestOptions{}
		err := options.Unmarshal(params)
		if err != nil {
			return nil, err
		}
		var reader io.Reader = nil
		if options.Method == "POST" {
			reader = bytes.NewReader([]byte(options.Body))
		}
		req, err := http.NewRequest(options.Method, options.Url, reader)
		if err != nil {
			return nil, err
		}
		if len(options.Headers) > 0 {
			for k, v := range options.Headers {
				req.Header.Set(k, v)
			}
		}
		if len(options.Cookies) > 0 {
			for k, v := range options.Cookies {
				req.AddCookie(&http.Cookie{
					Name:  k,
					Value: v,
				})
			}
		}
		client := http.Client{
			Jar: jar,
		}
		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				return
			}
		}(res.Body)
		headers := make(map[string]string)
		for key, value := range res.Header {
			if len(value) > 0 {
				headers[key] = value[0]
			}
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 50*1024*1024))
		if err != nil {
			return nil, err
		}
		response := &Response{
			Headers:  headers,
			BodyText: string(body),
		}
		responseJSON, err := response.Marshal()
		if err != nil {
			return nil, err
		}
		return r.ToValue(string(responseJSON)), nil
	}
}

type HTMLNode struct {
	Text       string            `json:"text"`
	Attributes map[string]string `json:"attributes"`
}

func FindAll(html string, selector string) ([]*HTMLNode, error) {
	var nodes []*HTMLNode
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte(html)))
	if err != nil {
		return nodes, err
	}
	doc.Find(selector).Each(func(i int, selection *goquery.Selection) {
		attributes := make(map[string]string)
		for _, attr := range selection.Get(0).Attr {
			attributes[attr.Key] = attr.Val
		}
		nodes = append(nodes, &HTMLNode{
			Text:       selection.Text(),
			Attributes: attributes,
		})
	})
	return nodes, err
}

func FindAllJS(r *goja.Runtime) func(this goja.Value, args ...goja.Value) (goja.Value, error) {
	return func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		html := this.String()
		if len(args) < 1 {
			return nil, fmt.Errorf("must provide a selector")
		}
		selector := args[0].String()
		nodes, err := FindAll(html, selector)
		if err != nil {
			return nil, err
		}
		jsonString, err := json.MarshalIndent(nodes, "", " ")
		if err != nil {
			return nil, err
		}
		return r.ToValue(string(jsonString)), nil
	}
}

func LogWithBot(r *goja.Runtime, b *gotgbot.Bot, ctx *ext.Context) func(this goja.Value, args ...goja.Value) (goja.Value, error) {
	return func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		SendMessage(b, this.String(), ctx.EffectiveMessage)
		return nil, nil
	}
}

func HookUpHTTPRequesting(r *goja.Runtime) (*goja.Runtime, error) {
	err := r.Set("request_go", RequestWithRuntime(r))
	return r, err
}

func HookUpHTMLParsing(r *goja.Runtime) (*goja.Runtime, error) {
	err := r.Set("findNodes_go", FindAllJS(r))
	return r, err
}

func HookUpLogWithBot(r *goja.Runtime, b *gotgbot.Bot, ctx *ext.Context) (*goja.Runtime, error) {
	err := r.Set("log", LogWithBot(r, b, ctx))
	return r, err
}

var JSDefinitions = getJSDefinitions()

func getJSDefinitions() string {
	var JSDefinitions = `
function request(options) {
	return JSON.parse(request_go(JSON.stringify(options)))
}
function findNodes(html, selector) {
	var nodes = findNodes_go(html, selector)
	return JSON.parse(nodes)
}
class URLParser{constructor(s){this.url=s}parse(){let s="",t="",l="",r="",e=this.url.split("://");if(e.length>1){s=e[0];let i=e[1];t=i.split("/")[0],l=i.substring(t.length)}else t=remainingUrl.split("/")[0],l=remainingUrl.substring(t.length);let n=l.indexOf("?");return -1!==n&&(r=l.substring(n+1),l=l.substring(0,n)),{protocol:s,host:t,path:l,query:r}}}
var Base64={_keyStr:"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",encode:function(e){var t="";var n,r,i,s,o,u,a;var f=0;e=Base64._utf8_encode(e);while(f<e.length){n=e.charCodeAt(f++);r=e.charCodeAt(f++);i=e.charCodeAt(f++);s=n>>2;o=(n&3)<<4|r>>4;u=(r&15)<<2|i>>6;a=i&63;if(isNaN(r)){u=a=64}else if(isNaN(i)){a=64}t=t+this._keyStr.charAt(s)+this._keyStr.charAt(o)+this._keyStr.charAt(u)+this._keyStr.charAt(a)}return t},decode:function(e){var t="";var n,r,i;var s,o,u,a;var f=0;e=e.replace(/[^A-Za-z0-9\+\/\=]/g,"");while(f<e.length){s=this._keyStr.indexOf(e.charAt(f++));o=this._keyStr.indexOf(e.charAt(f++));u=this._keyStr.indexOf(e.charAt(f++));a=this._keyStr.indexOf(e.charAt(f++));n=s<<2|o>>4;r=(o&15)<<4|u>>2;i=(u&3)<<6|a;t=t+String.fromCharCode(n);if(u!=64){t=t+String.fromCharCode(r)}if(a!=64){t=t+String.fromCharCode(i)}}t=Base64._utf8_decode(t);return t},_utf8_encode:function(e){e=e.replace(/\r\n/g,"\n");var t="";for(var n=0;n<e.length;n++){var r=e.charCodeAt(n);if(r<128){t+=String.fromCharCode(r)}else if(r>127&&r<2048){t+=String.fromCharCode(r>>6|192);t+=String.fromCharCode(r&63|128)}else{t+=String.fromCharCode(r>>12|224);t+=String.fromCharCode(r>>6&63|128);t+=String.fromCharCode(r&63|128)}}return t},_utf8_decode:function(e){var t="";var n=0;var r=c1=c2=0;while(n<e.length){r=e.charCodeAt(n);if(r<128){t+=String.fromCharCode(r);n++}else if(r>191&&r<224){c2=e.charCodeAt(n+1);t+=String.fromCharCode((r&31)<<6|c2&63);n+=2}else{c2=e.charCodeAt(n+1);c3=e.charCodeAt(n+2);t+=String.fromCharCode((r&15)<<12|(c2&63)<<6|c3&63);n+=3}}return t}}
function cleanFilename(e){return[/\$/g,/\&/g,/\^/g,/\?/g,/\</g,/\>/g,/\:/g,/\!/g,/\~/g,/\"/g,/\?/g].forEach(n=>{e=e.replace(n,"")}),e}
`
	JSDefinitions += "\nclass URLEncoder{static encode(e){return e.replace(/[^\\w]/gi,e=>`%${e.charCodeAt(0).toString(16)}`)}static decode(e){return e.replace(/%[\\dA-F]{2}/gi,e=>String.fromCharCode(parseInt(e.substring(1),16)))}}"
	return JSDefinitions
}

func CreateJSRuntime(secrets map[string]string, b *gotgbot.Bot, ctx *ext.Context) (*goja.Runtime, error) {
	r := goja.New()
	_, err := HookUpHTTPRequesting(r)
	if err != nil {
		return r, err
	}
	_, err = HookUpHTMLParsing(r)
	if err != nil {
		return r, err
	}
	_, err = HookUpLogWithBot(r, b, ctx)
	if err != nil {
		return r, err
	}
	_, err = r.RunString(JSDefinitions)
	if err != nil {
		return r, err
	}
	for k, v := range secrets {
		err = r.Set(k, v)
		if err != nil {
			return r, fmt.Errorf("error while setting up secrets %v", err)
		}
	}
	return r, nil
}
