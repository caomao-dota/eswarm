package util

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/plotozhu/MDCMainnet/common"
	"github.com/plotozhu/MDCMainnet/log"
	"github.com/plotozhu/MDCMainnet/rlp"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"sync"
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
	unreported ReportData
	reportMu   sync.Mutex
	account    [20]byte
	reportUrl string 

}

func CreateHttpReader( )(*HttpReader){
	client := createHTTPClient
	return &HttpReader{
		httpClient:client(),
		unreported:make(ReportData),
	}
}
func (hr *HttpReader)SetCdnReporter ( account, serverUrl string){
	hr.account = common.HexToAddress(account)
	hr.reportUrl = serverUrl +"/cdn"
}
//从中心化服务端取数据，最多取1024*1024*8 （8M字节数据）
const MaxLen  = 8*1024*1024
func (hr *HttpReader)GetChunkFromCentral(uri string,start int64,topHash []byte,r *http.Request) (result []byte, isEnd bool){
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
			if len(hr.reportUrl) != 0 {
				hr.doReport(hr.reportUrl,topHash,int64(len(result)))
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
func (s *HttpReader)GetDataFromCentralServer(uri string, r *http.Request,w http.ResponseWriter, hash []byte, onError OnError  ){


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
	if err != nil || response == nil || (response.StatusCode < 200 || response.StatusCode >= 300) {
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
		if len(s.reportUrl) != 0 {
			s.doReport(s.reportUrl,hash,int64(len(body)))
		}
		if w != nil {
			for k,vv := range response.Header {
				vv2 := make([]string, len(vv))
				copy(vv2, vv)
				if w.Header().Get(k) == ""  {
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

type RpData struct{
	Stime 	uint32
	MSec    uint32
	AmountH  uint32
	AmountL  uint32
}

type ReportData map[time.Time]int64
func (r *ReportData) EncodeRLP(w io.Writer) error {
	value := make([]RpData,0)
	for stime,amount := range *r {
		stime := stime.UnixNano()
		sec := stime/1e9
		msec := stime-sec
		data := RpData{uint32(sec),uint32(msec),uint32(amount>>32),uint32(amount)}
		value = append(value,data)
	}
	return rlp.Encode(w, value)
}
func (rd *ReportData)DecodeRLP(s *rlp.Stream) error{
	value := make([]*RpData,0)
	if err := s.Decode(&value); err != nil {
		return err
	}
	for _,res := range value {
		(*rd)[time.Unix(0,int64(res.Stime*1e9) + int64(res.MSec))] = (int64(res.AmountH) << 32 )+int64(res.AmountL)
	}

	return nil
}
type ReportFmt struct{
	Account [20]byte
	Hash    []byte
	Data    []byte
}
func (r *HttpReader) doReport(url string, hash []byte, dataLen int64) {
	r.reportMu.Lock()
	r.unreported[time.Now()] = dataLen
	r.reportMu.Unlock()
	go func(){
		r.reportMu.Lock()
		dataToReport := r.unreported
		r.unreported = make(ReportData)
		r.reportMu.Unlock()
		result,_ := rlp.EncodeToBytes(&dataToReport)
		data,_ := rlp.EncodeToBytes(ReportFmt{r.account,hash,result})
		err := SendDataToServer(url,5*time.Second,data)
		if err != nil {
			r.reportMu.Lock()
			for stime,amount := range dataToReport {
				r.unreported[stime] = amount
			}
			r.reportMu.Unlock()
		}
	}()
}
func  SendDataToServer(url string, timeout time.Duration, data []byte) error {

	client := &http.Client{
		Timeout: timeout,
	}

	request, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Info("error to send data","reason",err)
	}
	request.Header.Set("Connection", "Keep-Alive")
	request.Header.Set("Content-Type", "text/plain")

	res, err := client.Do(request)

	if err == nil { //提交成功，本地删除
		defer res.Body.Close()
		if res.StatusCode != 200 {
			err = errors.New(res.Status)
		}
	}
	return err
}