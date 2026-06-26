package designplan

import (
	"context"
	"errors"
)

type DynamicRecognizer struct {
	providerReader AIProviderReader
	readAsset      AssetReader
}

func NewDynamicRecognizerWithAssetReader(providerReader AIProviderReader, readAsset AssetReader) Recognizer {
	return &DynamicRecognizer{providerReader: providerReader, readAsset: readAsset}
}

func (r *DynamicRecognizer) Recognize(ctx context.Context, upload *UploadResult) (*RecognitionResult, error) {
	provider := ""
	if r.providerReader != nil {
		value, err := r.providerReader.GetAIProvider(ctx)
		if err != nil {
			return nil, err
		}
		provider = value
	}
	recognizer := NewRecognizerForProviderWithAssetReader(provider, r.readAsset)
	if recognizer == nil {
		return nil, errors.New("design plan recognizer is not configured")
	}
	return recognizer.Recognize(ctx, upload)
}
