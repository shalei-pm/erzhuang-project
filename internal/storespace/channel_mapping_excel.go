package storespace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const channelMappingExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

type exportImage struct {
	RowIndex int
	Ext      string
	Data     []byte
}

func buildChannelMappingExcel(ctx context.Context, rows []ChannelMappingExportRow, snapshotStore SnapshotStore) ([]byte, error) {
	var images []exportImage
	if snapshotStore != nil {
		for index, row := range rows {
			name := snapshotNameFromURL(row.SnapshotPath)
			if name == "" {
				continue
			}
			reader, contentType, err := snapshotStore.Open(ctx, name)
			if err != nil {
				continue
			}
			data, readErr := io.ReadAll(io.LimitReader(reader, maxSnapshotBytes+1))
			closeErr := reader.Close()
			if readErr != nil || closeErr != nil || len(data) == 0 || len(data) > maxSnapshotBytes {
				continue
			}
			images = append(images, exportImage{RowIndex: index + 2, Ext: excelImageExtension(contentType, name), Data: data})
		}
	}

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":        contentTypesXML(images),
		"_rels/.rels":                rootRelsXML(),
		"xl/workbook.xml":            workbookXML(),
		"xl/_rels/workbook.xml.rels": workbookRelsXML(),
		"xl/styles.xml":              stylesXML(),
		"xl/worksheets/sheet1.xml":   sheetXML(rows, len(images) > 0),
	}
	if len(images) > 0 {
		files["xl/worksheets/_rels/sheet1.xml.rels"] = sheetRelsXML()
		files["xl/drawings/drawing1.xml"] = drawingXML(images)
		files["xl/drawings/_rels/drawing1.xml.rels"] = drawingRelsXML(images)
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := addZipText(zipWriter, name, files[name]); err != nil {
			zipWriter.Close()
			return nil, err
		}
	}
	for index, image := range images {
		if err := addZipBinary(zipWriter, fmt.Sprintf("xl/media/image%d.%s", index+1, image.Ext), image.Data); err != nil {
			zipWriter.Close()
			return nil, err
		}
	}
	if err := zipWriter.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func sheetXML(rows []ChannelMappingExportRow, hasImages bool) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	builder.WriteString(`<cols><col min="1" max="1" width="8"/><col min="2" max="4" width="18"/><col min="5" max="6" width="14"/><col min="7" max="7" width="22"/><col min="8" max="9" width="16"/></cols>`)
	builder.WriteString(`<sheetData>`)
	builder.WriteString(excelRow(1, []string{"序号", "城市", "门店名称", "新氧机构ID", "录像机编号", "通道号", "最近截图", "业务区域类型", "编号/备注"}, 1, 22))
	for index, row := range rows {
		snapshotText := "无截图"
		if snapshotNameFromURL(row.SnapshotPath) != "" {
			snapshotText = "见图片"
		}
		builder.WriteString(excelRow(index+2, []string{
			strconv.Itoa(row.Index),
			row.City,
			row.StoreName,
			row.ExternalOrgID,
			row.RecorderCode,
			strconv.Itoa(row.ChannelNo),
			snapshotText,
			row.AreaTypeLabel,
			row.NumberOrNote,
		}, 0, 74))
	}
	builder.WriteString(`</sheetData>`)
	if hasImages {
		builder.WriteString(`<drawing r:id="rId1"/>`)
	}
	builder.WriteString(`</worksheet>`)
	return builder.String()
}

func excelRow(rowIndex int, values []string, style int, height int) string {
	var builder strings.Builder
	if height > 0 {
		builder.WriteString(fmt.Sprintf(`<row r="%d" ht="%d" customHeight="1">`, rowIndex, height))
	} else {
		builder.WriteString(fmt.Sprintf(`<row r="%d">`, rowIndex))
	}
	for index, value := range values {
		ref := fmt.Sprintf("%s%d", excelColumnName(index+1), rowIndex)
		styleAttr := ""
		if style > 0 {
			styleAttr = fmt.Sprintf(` s="%d"`, style)
		}
		builder.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"%s><is><t>%s</t></is></c>`, ref, styleAttr, xmlEscape(value)))
	}
	builder.WriteString(`</row>`)
	return builder.String()
}

func drawingXML(images []exportImage) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">`)
	for index, image := range images {
		row := image.RowIndex - 1
		builder.WriteString(`<xdr:twoCellAnchor editAs="oneCell">`)
		builder.WriteString(fmt.Sprintf(`<xdr:from><xdr:col>6</xdr:col><xdr:colOff>90000</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>90000</xdr:rowOff></xdr:from>`, row))
		builder.WriteString(fmt.Sprintf(`<xdr:to><xdr:col>7</xdr:col><xdr:colOff>1300000</xdr:colOff><xdr:row>%d</xdr:row><xdr:rowOff>780000</xdr:rowOff></xdr:to>`, row))
		builder.WriteString(fmt.Sprintf(`<xdr:pic><xdr:nvPicPr><xdr:cNvPr id="%d" name="截图%d"/><xdr:cNvPicPr/></xdr:nvPicPr>`, index+2, index+1))
		builder.WriteString(fmt.Sprintf(`<xdr:blipFill><a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill>`, index+1))
		builder.WriteString(`<xdr:spPr><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr></xdr:pic><xdr:clientData/></xdr:twoCellAnchor>`)
	}
	builder.WriteString(`</xdr:wsDr>`)
	return builder.String()
}

func contentTypesXML(images []exportImage) string {
	extensions := map[string]bool{}
	for _, image := range images {
		extensions[image.Ext] = true
	}
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	for _, ext := range []string{"jpg", "png", "webp"} {
		if extensions[ext] {
			contentType := "image/jpeg"
			if ext == "png" {
				contentType = "image/png"
			} else if ext == "webp" {
				contentType = "image/webp"
			}
			builder.WriteString(fmt.Sprintf(`<Default Extension="%s" ContentType="%s"/>`, ext, contentType))
		}
	}
	builder.WriteString(`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>`)
	builder.WriteString(`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`)
	builder.WriteString(`<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>`)
	if len(images) > 0 {
		builder.WriteString(`<Override PartName="/xl/drawings/drawing1.xml" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`)
	}
	builder.WriteString(`</Types>`)
	return builder.String()
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`
}

func workbookXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="通道映射确认表" sheetId="1" r:id="rId1"/></sheets></workbook>`
}

func workbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
}

func stylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Arial"/></font><font><b/><sz val="11"/><name val="Arial"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEFF6FF"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="1" borderId="0" xfId="0" applyFont="1" applyFill="1"/></cellXfs></styleSheet>`
}

func sheetRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/></Relationships>`
}

func drawingRelsXML(images []exportImage) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for index, image := range images {
		builder.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image%d.%s"/>`, index+1, index+1, image.Ext))
	}
	builder.WriteString(`</Relationships>`)
	return builder.String()
}

func addZipText(writer *zip.Writer, name string, value string) error {
	return addZipBinary(writer, name, []byte(value))
}

func addZipBinary(writer *zip.Writer, name string, value []byte) error {
	file, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = file.Write(value)
	return err
}

func xmlEscape(value string) string {
	var buffer bytes.Buffer
	if err := xml.EscapeText(&buffer, []byte(value)); err != nil {
		return ""
	}
	return buffer.String()
}

func excelColumnName(index int) string {
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func snapshotNameFromURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	name := filepath.Base(value)
	if validSnapshotName(name) {
		return name
	}
	return ""
}

func excelImageExtension(contentType string, name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	case "image/jpeg", "image/jpg":
		return "jpg"
	}
	if ext == "png" || ext == "webp" {
		return ext
	}
	return "jpg"
}
