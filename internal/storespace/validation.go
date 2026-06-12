package storespace

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type ValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

func validateCreateStoreInput(input CreateStoreInput) error {
	fields := map[string]string{}

	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "门店名称必填"
	}
	if strings.TrimSpace(input.City) == "" {
		fields["city"] = "城市必填"
	}

	hasDesignPlan := strings.TrimSpace(input.DesignPlanUploadID) != ""
	hasRecorder := false
	if len(input.Recorders) > 3 {
		fields["recorders"] = "单门店最多 3 台录像机"
	}
	seenCodes := map[string]bool{}
	for index, recorder := range input.Recorders {
		prefix := fmt.Sprintf("recorders[%d]", index)
		code := normalizeDeviceCode(recorder.DeviceCode)
		if code == "" {
			continue
		}
		hasRecorder = true
		if seenCodes[code] {
			fields[prefix+".device_code"] = "同一门店内录像机设备编码不能重复"
		}
		seenCodes[code] = true
	}
	if !hasDesignPlan && !hasRecorder {
		fields["resources"] = "请至少上传设计图或填写一个录像机设备编码"
	}

	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateCreateEzvizAccountInput(input CreateEzvizAccountInput) error {
	fields := map[string]string{}
	if strings.TrimSpace(input.AccountName) == "" {
		fields["account_name"] = "账号名称必填"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateAddRecorderInput(input AddRecorderInput) error {
	fields := map[string]string{}
	if normalizeDeviceCode(input.DeviceCode) == "" {
		fields["device_code"] = "录像机设备编码必填"
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateAreaLookup(input AreaLookup) (int, error) {
	fields := map[string]string{}
	if input.StoreID <= 0 {
		fields["store_id"] = "门店 ID 必填"
	}
	if !validAreaType(input.Type) {
		fields["area_type"] = "区域类型只能是 treatment、consultation、beauty"
	}
	numberText := strings.TrimSpace(input.NumberText)
	if numberText == "" {
		fields["area_number"] = "区域编号必填"
	} else if !onlyDigits(numberText) {
		fields["area_number"] = "区域编号只能是数字"
	}
	if input.Source != "" && !validAreaSource(input.Source) {
		fields["source"] = "区域来源只能是 manual、design_plan、video_channel、multiple"
	}
	if len(fields) > 0 {
		return 0, &ValidationError{Fields: fields}
	}
	number, err := strconv.Atoi(numberText)
	if err != nil || number <= 0 {
		return 0, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
	}
	return number, nil
}

func validAreaType(value AreaType) bool {
	return value == AreaTypeTreatment || value == AreaTypeConsultation || value == AreaTypeBeauty
}

func validAreaSource(value AreaSource) bool {
	return value == AreaSourceManual ||
		value == AreaSourceDesignPlan ||
		value == AreaSourceVideoChannel ||
		value == AreaSourceMultiple
}

func onlyDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func normalizeDeviceCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
