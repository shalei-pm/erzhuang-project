package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/shalei-pm/erzhuang-project/internal/channelai"
	"github.com/shalei-pm/erzhuang-project/internal/designplan"
)

func main() {
	mode := flag.String("mode", "channel", "smoke mode: channel or design")
	imageURL := flag.String("image-url", "", "public snapshot URL for channel recognition")
	imageFile := flag.String("image-file", "", "local PNG/JPEG file for design plan recognition")
	timeout := flag.Duration("timeout", 90*time.Second, "request timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	startedAt := time.Now()
	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "channel":
		if err := smokeChannel(ctx, *imageURL); err != nil {
			log.Fatalf("channel smoke failed: %v", err)
		}
	case "design":
		if err := smokeDesign(ctx, *imageFile); err != nil {
			log.Fatalf("design smoke failed: %v", err)
		}
	default:
		log.Fatalf("unsupported --mode %q", *mode)
	}
	fmt.Printf("elapsed_ms=%d\n", time.Since(startedAt).Milliseconds())
}

func smokeChannel(ctx context.Context, imageURL string) error {
	if strings.TrimSpace(imageURL) == "" {
		return fmt.Errorf("--image-url is required for channel mode")
	}
	recognizer, enabled, err := channelai.NewRecognizerFromEnv()
	if err != nil {
		return err
	}
	if !enabled {
		return fmt.Errorf("channel recognizer is not enabled; configure OPENAI_API_KEY/VISION_API_KEY or CHANNEL_AI_PROVIDER=minimax with MINIMAX_API_KEY")
	}
	result, err := recognizer.Recognize(ctx, imageURL)
	if err != nil {
		return err
	}
	fmt.Printf("provider=%s\n", result.Provider)
	fmt.Printf("scene_type=%s\n", result.SceneType)
	fmt.Printf("area_type=%s\n", result.AreaType)
	fmt.Printf("area_number=%s\n", result.AreaNumber)
	fmt.Printf("card_text=%s\n", result.CardText)
	fmt.Printf("confidence=%s\n", result.Confidence)
	fmt.Printf("needs_review=%t\n", result.NeedsReview)
	fmt.Printf("raw_notes=%s\n", result.RawNotes)
	return nil
}

func smokeDesign(ctx context.Context, imageFile string) error {
	if strings.TrimSpace(imageFile) == "" {
		return fmt.Errorf("--image-file is required for design mode")
	}
	recognizer := designplan.NewRecognizerFromEnvWithAssetReader(func(value string) (io.ReadCloser, string, error) {
		file, err := os.Open(value)
		if err != nil {
			return nil, "", err
		}
		return file, contentTypeFromName(value), nil
	})
	result, err := recognizer.Recognize(ctx, &designplan.UploadResult{PreviewPath: imageFile})
	if err != nil {
		return err
	}
	fmt.Printf("store_name=%s\n", result.StoreName)
	fmt.Printf("store_name_confidence=%s\n", result.StoreNameConfidence)
	fmt.Printf("area_count=%d\n", len(result.Areas))
	for index, area := range result.Areas {
		fmt.Printf("area_%d=%s,%s,%s,%s,needs_review=%t\n", index+1, area.Name, area.Type, area.Number, area.Confidence, area.NeedsReview)
	}
	fmt.Printf("raw_notes=%s\n", result.RawNotes)
	return nil
}

func contentTypeFromName(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return "image/png"
	}
}
