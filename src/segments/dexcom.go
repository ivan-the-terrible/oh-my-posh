package segments

import (
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/jandedobbeleer/oh-my-posh/src/properties"
	"github.com/jandedobbeleer/oh-my-posh/src/runtime"
	"github.com/jandedobbeleer/oh-my-posh/src/runtime/http"
)

type Dexcom struct {
	props properties.Properties
	env   runtime.Environment

	api   DexcomAPI
	DexcomData
	TrendIcon string
}

// DexcomData struct contains the API data, just the data we need although there is more
type DexcomData struct {
	Sgv        int    `json:"value"`
	Status     string `json:"status"`
	Trend      string `json:"trend"`
}

type DexcomEGVSData struct {
	Records []DexcomData `json:"records"`
}

const (
	DexcomAccessTokenKey = "dexcom_access_token"
	DexcomRefreshTokenKey = "dexcom_refresh_token"
)

// https://developer.dexcom.com/docs/dexcomv3/endpoint-overview/
type DexcomAPI interface {
	GetEstimatedGlucoseValues() (*DexcomEGVSData, error)
}

type dexcomAPI struct {
	*http.OAuthRequest
}

func (d *dexcomAPI) GetEstimatedGlucoseValues() (*DexcomEGVSData, error) {
	return d.getDexcomAPIData("egvs")
}

func (d *dexcomAPI) getDexcomAPIData(endpoint string) (*DexcomEGVSData, error) {
	urlStr := "https://api.dexcom.com/v3/users/self/" + endpoint

	apiURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	params := apiURL.Query()

	now := time.Now()

	startDate := now.Add(-5 * time.Minute).Format("2020-01-04T07:41:55") // 5 minutes ago
	endDate := now.Format("2020-01-04T07:46:55")

	params.Add("startDate", startDate)
	params.Add("endDate", endDate)

	url := apiURL.String()

	data, err := http.OauthResult[*DexcomEGVSData](d.OAuthRequest, url, nil)

	return data, err
}

// Segment template, where SGV is Sensor Glucose Value
func (d *Dexcom) Template() string {
	return " {{ .Sgv }} "
}

func (d *Dexcom) Enabled() bool {
	data, err := d.getResult()
	if err != nil {
		return false
	}

	d.DexcomData = *data
	d.TrendIcon = d.getTrendIcon()

	return true
}

func (d *Dexcom) getTrendIcon() string {
	switch d.Trend {
	case "doubleUp":
		return "↑↑"
	case "singleUp":
		return "↑"
	case "fortyFiveUp":
		return "↗"
	case "flat":
		return "→"
	case "fortyFiveDown":
		return "↘"
	case "singleDown":
		return "↓"
	case "doubleDown":
		return "↓↓"
	default:
		return ""
	}
}

func (d *Dexcom) getResult() (*DexcomData, error) {
	getCacheValue := func(key string) (*DexcomData, error) {
		val, found := d.env.Cache().Get(key)
		// we got something from the cache
		if found {
			var data *DexcomData
			err := json.Unmarshal([]byte(val), &data)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
		return nil, errors.New("no data in cache")
	}

	cacheTimeout := d.props.GetInt(properties.CacheTimeout, 5)

	if cacheTimeout > 0 {
		if data, err := getCacheValue("dexcom"); err == nil {
			return data, nil
		}
	}

	egvsRecords, err := d.api.GetEstimatedGlucoseValues()
	if err != nil {
		return nil, err
	}


	if len(egvsRecords.Records) == 0 {
		return nil, errors.New("no records found")
	}
	// first record is the most recent
	data := &egvsRecords.Records[0]

	if cacheTimeout > 0 {
		// persist new sugars in cache
		dataJSON, err := json.Marshal(data)
		if err == nil {
			d.env.Cache().Set("dexcom", string(dataJSON), cacheTimeout)
		}
	}

	return data, nil
}

func (d *Dexcom) Init(props properties.Properties, env runtime.Environment) {
	d.props = props
	d.env = env

	oauth := &http.OAuthRequest{
		AccessTokenKey: DexcomAccessTokenKey,
		RefreshTokenKey: DexcomRefreshTokenKey,
		SegmentName: "dexcom",
		AccessToken: props.GetString(DexcomAccessTokenKey, ""),
		RefreshToken: props.GetString(DexcomRefreshTokenKey, ""),
		Request: http.Request{
			Env: env,
			CacheTimeout: props.GetInt(properties.CacheTimeout, 5),
			HTTPTimeout: props.GetInt(properties.HTTPTimeout, properties.DefaultHTTPTimeout),
		},
	}

	d.api = &dexcomAPI{
		OAuthRequest: oauth,
	}
}
