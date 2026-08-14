package designplan

import (
	"fmt"
	"strings"
	"unicode"
)

type ValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

func ValidateStoreInput(input StoreInput) error {
	fields := map[string]string{}

	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "门店名必填"
	}
	if input.Status != "" && !validStoreStatus(input.Status) {
		fields["status"] = "配置状态只能是 completed、needs_review、incomplete"
	}
	if len(input.Areas) == 0 {
		fields["areas"] = "至少需要 1 个区域"
	}

	seenNumbers := map[string]bool{}
	for index, area := range input.Areas {
		prefix := fmt.Sprintf("areas[%d]", index)
		if strings.TrimSpace(area.Name) == "" {
			fields[prefix+".name"] = "区域名称必填"
		}
		if !validAreaType(area.Type) {
			fields[prefix+".type"] = "区域类型必填，且只能是 treatment、vip_treatment、consultation、beauty"
		}
		if area.Box == nil {
			fields[prefix+".box"] = "区域框必填"
		} else if !validBox(*area.Box) {
			fields[prefix+".box"] = "区域框必须是 0 到 1 的比例坐标，且宽高大于 0"
		}
		if area.Confidence != "" && !validConfidence(area.Confidence) {
			fields[prefix+".confidence"] = "置信度只能是 high、medium、low"
		}

		number := strings.TrimSpace(string(area.Number))
		if area.Type == AreaTypeTreatment || area.Type == AreaTypeConsultation {
			if number == "" {
				fields[prefix+".number"] = "治疗室/面诊室编号必填"
			}
		}
		if number != "" && !onlyDigits(number) {
			fields[prefix+".number"] = "编号只能是数字"
		}
		if number != "" && validAreaType(area.Type) {
			key := string(area.Type) + ":" + number
			if seenNumbers[key] {
				fields[prefix+".number"] = "同门店同类型编号必须唯一"
			}
			seenNumbers[key] = true
		}
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validAreaType(areaType AreaType) bool {
	return areaType == AreaTypeTreatment ||
		areaType == AreaTypeVIPTreatment ||
		areaType == AreaTypeConsultation ||
		areaType == AreaTypeBeauty
}

func validConfidence(confidence Confidence) bool {
	return confidence == ConfidenceHigh ||
		confidence == ConfidenceMedium ||
		confidence == ConfidenceLow
}

func validStoreStatus(status StoreStatus) bool {
	return status == StoreStatusCompleted ||
		status == StoreStatusNeedsReview ||
		status == StoreStatusIncomplete
}

func validBox(box Box) bool {
	return box.X >= 0 &&
		box.Y >= 0 &&
		box.Width > 0 &&
		box.Height > 0 &&
		box.X <= 1 &&
		box.Y <= 1 &&
		box.Width <= 1 &&
		box.Height <= 1 &&
		box.X+box.Width <= 1 &&
		box.Y+box.Height <= 1
}

func onlyDigits(value string) bool {
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return value != ""
}
