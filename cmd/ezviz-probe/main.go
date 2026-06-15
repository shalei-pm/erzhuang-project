package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/ezviz"
)

func main() {
	dataPath := flag.String("data", "", "path to local ezviz markdown data file")
	region := flag.String("region", "华北", "region name from markdown data")
	captureLimit := flag.Int("capture-limit", 3, "number of channels to capture")
	flag.Parse()

	if *dataPath == "" {
		log.Fatal("--data is required")
	}

	source, err := os.ReadFile(*dataPath)
	if err != nil {
		log.Fatalf("read data file: %v", err)
	}
	accounts, err := ezviz.ParseAccountsMarkdown(source)
	if err != nil {
		log.Fatalf("parse data file: %v", err)
	}
	selected, ok := ezviz.FindAccountByRegion(accounts, *region)
	if !ok {
		log.Fatalf("region %q not found", *region)
	}

	client := ezviz.NewClient(ezviz.ClientOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Printf("region: %s\n", selected.Region)
	fmt.Printf("account: %s\n", selected.Account.Name)
	fmt.Printf("deviceSerial: %s\n", selected.DeviceSerial)

	cameras, err := client.CameraList(ctx, selected.Account, selected.DeviceSerial)
	if err != nil {
		log.Fatalf("camera list failed: %v", err)
	}
	sort.Slice(cameras, func(i, j int) bool {
		return cameras[i].ChannelNo < cameras[j].ChannelNo
	})
	fmt.Printf("cameraList: ok, channels=%d\n", len(cameras))
	activeCameras := make([]ezviz.Camera, 0, len(cameras))
	for _, camera := range cameras {
		fmt.Printf("channel: no=%d name=%q status=%d\n", camera.ChannelNo, camera.CameraName, camera.Status)
		if camera.Status == 1 {
			activeCameras = append(activeCameras, camera)
		}
	}
	fmt.Printf("activeChannels: %d\n", len(activeCameras))

	limit := *captureLimit
	if limit < 0 {
		limit = 0
	}
	if limit > len(activeCameras) {
		limit = len(activeCameras)
	}
	for index := 0; index < limit; index++ {
		camera := activeCameras[index]
		result, err := client.Capture(ctx, selected.Account, selected.DeviceSerial, camera.ChannelNo)
		if err != nil {
			fmt.Printf("capture: no=%d result=failed error=%v\n", camera.ChannelNo, err)
			continue
		}
		fmt.Printf("capture: no=%d result=ok picUrl=%s\n", camera.ChannelNo, redactURL(result.PicURL))
		if index+1 < limit {
			time.Sleep(4 * time.Second)
		}
	}
}

func redactURL(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 48 {
		return "[pic-url-present]"
	}
	return value[:32] + "...[redacted]..." + value[len(value)-12:]
}
