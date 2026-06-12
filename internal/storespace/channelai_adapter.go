package storespace

import (
	"context"

	"github.com/shalei-pm/erzhuang-project/internal/channelai"
)

type ChannelAIAdapter struct {
	recognizer channelai.Recognizer
}

func NewChannelAIAdapter(recognizer channelai.Recognizer) *ChannelAIAdapter {
	return &ChannelAIAdapter{recognizer: recognizer}
}

func (a *ChannelAIAdapter) RecognizeChannel(ctx context.Context, imageURL string) (ChannelRecognitionResult, error) {
	result, err := a.recognizer.Recognize(ctx, imageURL)
	if err != nil {
		return ChannelRecognitionResult{}, err
	}
	rawResult := ""
	if len(result.RawResult) > 0 {
		rawResult = string(result.RawResult)
	}
	return ChannelRecognitionResult{
		SceneType:      result.SceneType,
		AreaType:       result.AreaType,
		AreaNumber:     result.AreaNumber,
		CardText:       result.CardText,
		DecisionSource: result.DecisionSource,
		Confidence:     result.Confidence,
		NeedsReview:    result.NeedsReview,
		RawNotes:       result.RawNotes,
		Provider:       result.Provider,
		RawResult:      rawResult,
	}, nil
}
