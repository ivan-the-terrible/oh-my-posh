package segments

import (
	"encoding/json"
	"errors"
	"testing"

	cache_ "github.com/jandedobbeleer/oh-my-posh/src/cache/mock"
	"github.com/jandedobbeleer/oh-my-posh/src/properties"
	"github.com/jandedobbeleer/oh-my-posh/src/runtime/mock"
	"github.com/stretchr/testify/assert"
	testify_ "github.com/stretchr/testify/mock"
)

type mockedDexcomAPI struct {
	testify_.Mock
}

func (d *mockedDexcomAPI) GetEstimatedGlucoseValues() (*DexcomEGVSData, error) {
	args := d.Called()
	return args.Get(0).(*DexcomEGVSData), args.Error(1)
}

func TestDexcomSegment(t *testing.T) {
	cases := []struct {
		Case            string
		DexcomData      *DexcomEGVSData
		ExpectedString  string
		ExpectedEnabled bool
		CacheTimeout    int
		CacheFoundFail  bool
		Template        string
		Error           error
	}{
		{
			Case:       "Flat 150 from cache not found",
			DexcomData: &DexcomEGVSData{
				Records: []DexcomData{
					{
						Sgv:    150,
						Status: "ok",
						Trend:  "flat",
					},
				},
			},
			Template:        "\ue2a1 {{.Sgv}}{{.TrendIcon}}",
			ExpectedString:  "\ue2a1 150→",
			ExpectedEnabled: true,
			CacheFoundFail:  true,
			CacheTimeout:    5,
		},
		{
			Case:       "DoubleDown 50 from cache",
			DexcomData: &DexcomEGVSData{
				Records: []DexcomData{
					{
						Sgv:    50,
						Status: "ok",
						Trend:  "doubleDown",
					},
				},
			},
			Template:        "\ue2a1 {{.Sgv}}{{.TrendIcon}}",
			ExpectedString:  "\ue2a1 50↓↓",
			ExpectedEnabled: true,
			CacheTimeout:    5,
		},
		{
			Case:            "Empty array",
			DexcomData:      &DexcomEGVSData{},
			ExpectedEnabled: false,
		},
		{
			Case:            "Error in retrieving data",
			DexcomData:      &DexcomEGVSData{},
			Error:           errors.New("Something went wrong"),
			ExpectedEnabled: false,
		},
	}

	for _, tc := range cases {
		env := &mock.Environment{}

		cache := &cache_.Cache{}
		data_cache := ""
		if len(tc.DexcomData.Records) != 0 {
			data, err := json.Marshal(&tc.DexcomData.Records[0])
			if err == nil {
				data_cache = string(data)
			}
		}
		cache.On("Get", "dexcom").Return(data_cache, !tc.CacheFoundFail)
		cache.On("Set", "dexcom", data_cache, tc.CacheTimeout).Return()
		env.On("Cache").Return(cache)

		api := &mockedDexcomAPI{}
		api.On("GetEstimatedGlucoseValues").Return(tc.DexcomData, tc.Error)

		dexcom := &Dexcom{
			api: api,
			props: &properties.Map{},
			env: env,
		}

		enabled := dexcom.Enabled()
		assert.Equal(t, tc.ExpectedEnabled, enabled, tc.Case)
		if !enabled {
			continue
		}

		if tc.Template == "" {
			tc.Template = dexcom.Template()
		}

		var got = renderTemplate(&mock.Environment{}, tc.Template, dexcom)
		assert.Equal(t, tc.ExpectedString, got, tc.Case)
	}
}
