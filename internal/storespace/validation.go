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

	validateStoreBasicFields(input.City, input.Name, fields)

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

func validateUpdateStoreBasicInfoInput(input UpdateStoreBasicInfoInput) error {
	fields := map[string]string{}
	validateStoreBasicFields(input.City, input.Name, fields)
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateStoreBasicFields(city string, name string, fields map[string]string) {
	if strings.TrimSpace(name) == "" {
		fields["name"] = "门店名称必填"
	}
	if strings.TrimSpace(city) == "" {
		fields["city"] = "城市必填"
	}
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

func validateSaveDesignPlanInput(input SaveDesignPlanInput) error {
	fields := map[string]string{}
	if strings.TrimSpace(input.PreviewImagePath) == "" && strings.TrimSpace(input.UploadID) == "" {
		fields["design_plan"] = "请先上传设计图"
	}
	if len(input.Areas) == 0 {
		fields["areas"] = "至少需要维护 1 个业务区域"
	}
	seenNumbers := map[string]bool{}
	for index, area := range input.Areas {
		prefix := fmt.Sprintf("areas[%d]", index)
		if !validAreaType(area.Type) {
			fields[prefix+".area_type"] = "区域类型只能是 treatment、vip_treatment、consultation、beauty"
		}
		numberText := strings.TrimSpace(area.NumberText)
		number := mustPositiveInt(numberText)
		if numberText == "" {
			if area.Type != AreaTypeVIPTreatment {
				fields[prefix+".area_number"] = "区域编号必填"
			}
		} else if !onlyDigits(area.NumberText) {
			fields[prefix+".area_number"] = "区域编号只能是数字"
		} else if number <= 0 {
			fields[prefix+".area_number"] = "区域编号必须是正整数"
		}
		if area.Box == nil {
			fields[prefix+".box"] = "高亮框不能为空"
		}
		if area.Type != "" && (number > 0 || (area.Type == AreaTypeVIPTreatment && numberText == "")) {
			key := string(area.Type) + ":" + strconv.Itoa(number)
			if seenNumbers[key] {
				fields[prefix+".area_number"] = "同类型下编号不能重复"
			}
			seenNumbers[key] = true
		}
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
		fields["area_type"] = "区域类型只能是 treatment、vip_treatment、consultation、beauty"
	}
	numberText := strings.TrimSpace(input.NumberText)
	if numberText == "" {
		if input.Type != AreaTypeVIPTreatment {
			fields["area_number"] = "区域编号必填"
		}
	} else if !onlyDigits(numberText) {
		fields["area_number"] = "区域编号只能是数字"
	}
	if input.Source != "" && !validAreaSource(input.Source) {
		fields["source"] = "区域来源只能是 manual、design_plan、video_channel、multiple"
	}
	if len(fields) > 0 {
		return 0, &ValidationError{Fields: fields}
	}
	if numberText == "" && input.Type == AreaTypeVIPTreatment {
		return 0, nil
	}
	number, err := strconv.Atoi(numberText)
	if err != nil || number <= 0 {
		return 0, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
	}
	return number, nil
}

func mustPositiveInt(value string) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || number < 0 {
		return 0
	}
	return number
}

func validateChannelConfirmationInput(input ChannelConfirmationInput) (int, error) {
	fields := map[string]string{}
	kind := strings.TrimSpace(input.Kind)
	if kind != "" && kind != "business" && kind != "non_business" {
		fields["kind"] = "确认类型只能是 business 或 non_business"
	}
	if input.AreaType == "" {
		if kind == "business" {
			fields["area_type"] = "业务区域类型必填"
		}
		if input.SceneType != "" && !validSceneType(input.SceneType) {
			fields["scene_type"] = "场景类型不合法"
		}
		if len([]rune(strings.TrimSpace(input.AreaNote))) > 40 {
			fields["area_note"] = "备注不能超过 40 个字"
		}
		if len(fields) > 0 {
			return 0, &ValidationError{Fields: fields}
		}
		return 0, nil
	}
	if !validAreaType(input.AreaType) {
		fields["area_type"] = "区域类型只能是 treatment、vip_treatment、consultation、beauty"
	}
	numberText := strings.TrimSpace(input.AreaNumber)
	if numberText == "" {
		if input.AreaType != AreaTypeVIPTreatment {
			fields["area_number"] = "区域编号必填"
		}
	} else if !onlyDigits(numberText) {
		fields["area_number"] = "区域编号只能是数字"
	}
	if len(fields) > 0 {
		return 0, &ValidationError{Fields: fields}
	}
	number, err := strconv.Atoi(numberText)
	if numberText == "" && input.AreaType == AreaTypeVIPTreatment {
		return 0, nil
	}
	if err != nil || number <= 0 {
		return 0, &ValidationError{Fields: map[string]string{"area_number": "区域编号必须是正整数"}}
	}
	return number, nil
}

func validAreaType(value AreaType) bool {
	return value == AreaTypeTreatment || value == AreaTypeVIPTreatment || value == AreaTypeConsultation || value == AreaTypeBeauty
}

func validAreaSource(value AreaSource) bool {
	return value == AreaSourceManual ||
		value == AreaSourceDesignPlan ||
		value == AreaSourceVideoChannel ||
		value == AreaSourceMultiple
}

func validSceneType(value SceneType) bool {
	switch value {
	case SceneTypeTreatment,
		SceneTypeVIPTreatment,
		SceneTypeConsultation,
		SceneTypeBeauty,
		SceneTypeFrontDesk,
		SceneTypeCorridor,
		SceneTypePassage,
		SceneTypeWaitingArea,
		SceneTypeHall,
		SceneTypeEntrance,
		SceneTypeStorage,
		SceneTypePharmacy,
		SceneTypeMachineRoom,
		SceneTypeUnknown:
		return true
	default:
		return false
	}
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
