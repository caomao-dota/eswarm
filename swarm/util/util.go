package util

import (
	"bytes"
	"fmt"
	"github.com/plotozhu/MDCMainnet/log"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// createHTTPClient for connection re-use
func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 300 * time.Second,
			}).DialContext,
			MaxIdleConns:        2,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:	 1000 * time.Second,
		},
	}
	return client
}

type HttpReader struct {
	httpClient *http.Client
}

func CreateHttpReader()(*HttpReader){
	client := createHTTPClient
	return &HttpReader{
		httpClient:client(),
	}
}
//从中心化服务端取数据，最多取1024*1024*8 （8M字节数据）
const MaxLen  = 8*1024*1024
func (hr *HttpReader)GetChunkFromCentral(uri string,start int64,r *http.Request) (result []byte, isEnd bool){
	req, err := http.NewRequest("GET", uri, bytes.NewBuffer([]byte("")))
	if err != nil {
		result = nil
		isEnd = true
		return
	}
	for k,vv := range r.Header {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		req.Header[k] = vv2
	}

	req.Header["Range"] = []string{fmt.Sprintf("bytes=%v-%v",start,start+MaxLen )}
	// use httpClient to send request
	response, err := hr.httpClient.Do(req)
	if err == nil && response != nil {

		// Close the connection to reuse it
		defer response.Body.Close()

		// Let's check if the work actually is done
		// We have seen inconsistencies even when we get 200 OK response
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Error("Couldn't parse response body. %+v", err)
		}
		length := int64(len(body))
		if length > MaxLen {
			length = MaxLen
		}
		result = body
		contentRange := response.Header["Content-Range"]
		if len(contentRange) > 0 {
			patternM3u8 := regexp.MustCompile(`bytes\s*[0-9]+-(?P<end>[0-9]+)\/(?P<total>[0-9]+)`)
			matchResult := patternM3u8.FindSubmatch([]byte(contentRange[0]))
			if len(matchResult) > 2 {
				endByte,err1 := strconv.ParseInt(string(matchResult[1]), 10, 64)
				totalByte,err2 :=  strconv.ParseInt(string(matchResult[2]), 10, 64)
				if err1 == nil && err2 == nil && endByte +1 != totalByte {
					isEnd = false
				}else {
					isEnd = true
				}
			}else{
				isEnd = true
			}


		}else{
			isEnd = true
		}
		return
		//log.Trace("Response Body:", string(body))
	}else{
		result = nil
		isEnd = true
		return
	}
}
type OnError func(http.ResponseWriter, *http.Request,  string,  int)
//get data from server and write to response
func (s *HttpReader)GetDataFromCentralServer(uri string, r *http.Request,w http.ResponseWriter, onError OnError  ){


	req, err := http.NewRequest("GET", uri, bytes.NewBuffer([]byte("")))
	if err != nil {
		log.Error("Error Occured. %+v", err)
		onError(w, r, fmt.Sprintf("Error Occured :%+v", err), http.StatusInternalServerError)
		return
	}
	for k,vv := range r.Header {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		req.Header[k] = vv2
	}

	// use httpClient to send request
	response, err := s.httpClient.Do(req)
	if err != nil && response == nil {
		log.Error("Error sending request to API endpoint. %+v", "error:", err)
		onError(w, r, fmt.Sprintf("Error Occured :%+v", err), http.StatusBadRequest)
	} else {
		// Close the connection to reuse it
		defer response.Body.Close()

		// Let's check if the work actually is done
		// We have seen inconsistencies even when we get 200 OK response
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Error("Couldn't parse response body. %+v", err)
		}

		if w != nil {
			for k,vv := range response.Header {
				vv2 := make([]string, len(vv))
				copy(vv2, vv)
				if w.Header().Get(k) == "" && k == "Content-Type" {
					w.Header().Set(k,vv[0])
				}

				//	w.Header().Set(k,vv2)
				//log.Trace(k,vv)
			}
			w.Write(body)
		}

		return
		//log.Trace("Response Body:", string(body))
	}
}