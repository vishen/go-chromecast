package application

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/vishen/go-chromecast/cast"
)

// getInfo uses the http://<ip>:8008/setup/eureka_endpoint to obtain more
// information about the cast-device.
// OBS: The 8008 seems to be pure http, whereas 8009 is typically the port
// to use for protobuf-communication,
func GetInfo(ip string) (info *cast.DeviceInfo, err error) {
	url := fmt.Sprintf("http://%v:8008/setup/eureka_info", ip)
	log.Printf("Fetching: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	info = new(cast.DeviceInfo)
	if err := json.Unmarshal(data, info); err != nil {
		return nil, err
	}
	return info, nil
}
