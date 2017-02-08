package media_library

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	// Package imaging provides basic image manipulation functions (resize, rotate, flip, crop, etc.). This package is based on the standard Go image package and works best along with it.
	"github.com/disintegration/imaging"
	//  The fantastic ORM library for Golang, aims to be developer friendly.
	"github.com/jinzhu/gorm"
	//	a golang library using Common Locale Data Repository to format dates
	"github.com/jinzhu/inflection"

	"github.com/Sky-And-Hammer/TM_EC"
	"github.com/Sky-And-Hammer/TM_EC/resource"
	"github.com/Sky-And-Hammer/TM_EC/utils"
	"github.com/Sky-And-Hammer/admin"
)

type CropOption struct {
	X, Y, Width, Height int
}

type FileHeader interface {
	Open() (multipart.File, error)
}

type fileWrapper struct {
	*os.File
}

func (fileWrapper *fileWrapper) Open() (multipart.File, error) {
	return fileWrapper.File, nil
}

type Base struct {
	FileName    string
	Url         string
	CropOptions map[string]*CropOption `json:",omitempty"`
	Delete      bool                   `json:",omitempty"`
	Crop        bool                   `json:"-"`
	FileHeader  FileHeader             `json:"-"`
	Reader      io.Reader              `json:"-"`
	cropped     bool
}

func (b *Base) Scan(data interface{}) (err error) {
	switch values := data.(type) {
	case *os.File:
		b.FileHeader, b.FileName = &fileWrapper{values}, path.Base(values.Name())
	case *multipart.FileHeader:
		b.FileHeader, b.FileName = values, values.Filename
	case []*multipart.FileHeader:
		if len(values) > 0 {
			file := values[0]
			b.FileHeader, b.FileName = file, file.Filename
		}
	case []byte:
		if string(values) != "" {
			if err = json.Unmarshal(values, b); err == nil {
				var doCrop struct{ Crop bool }
				if err = json.Unmarshal(values, &doCrop); err == nil && doCrop.Crop {
					b.Crop = true
				}
			}
		}
	case string:
		return b.Scan([]byte(values))
	case []string:
		for _, str := range values {
			if err := b.Scan(str); err != nil {
				return err
			}
		}
	default:
		err = errors.New("unsupported driver -> Scan pair for MediaLibrary")
	}

	if b.Delete {
		b.Url = ""
		b.FileName = ""
		b.CropOptions = nil
	}
	return
}

func (b *Base) Value() (driver.Value, error) {
	if b.Delete {
		return nil, nil
	}
	result, err := json.Marshal(b)
	return string(result), err
}

func (b *Base) Ext() string {
	return strings.ToLower(path.Ext(b.Url))
}

func (b *Base) URL(styles ...string) string {
	if b.Url != "" && len(styles) > 0 {
		ext := path.Ext(b.Url)
		return fmt.Sprintf("%v.%v%v", strings.TrimSuffix(b.Url, ext), styles[0], ext)
	}
	return b.Url
}

func (b *Base) String() string {
	return b.URL()
}

func (b *Base) GetFileName() string {
	return b.FileName
}

func (b *Base) GetFieldHeader() FileHeader {
	return b.FileHeader
}

func (b *Base) GetURLTemplate(option *Option) (path string) {
	if path = option.Get("URL"); path == "" {
		path = "/system/{{class}}/{{primary_key}}/{{column}}/{{filename_with_hash}}"
	}
	return
}

var urlReplacer = regexp.MustCompile("(\\s|\\+)+")

func getFuncMap(scope *gorm.Scope, field *gorm.Field, filename string) template.FuncMap {
	hash := func() string { return strings.Replace(time.Now().Format("20060102150506.000000000"), ".", "", -1) }
	return template.FuncMap{
		"class":       func() string { return inflection.Plural(utils.ToParamString(scope.GetModelStruct().ModelType.Name())) },
		"primary_key": func() string { return fmt.Sprintf("%v", scope.PrimaryKeyValue()) },
		"column":      func() string { return strings.ToLower(field.Name) },
		"filename":    func() string { return filename },
		"basename":    func() string { return strings.TrimSuffix(path.Base(filename), path.Ext(filename)) },
		"hash":        hash,
		"filename_with_hash": func() string {
			return urlReplacer.ReplaceAllString(fmt.Sprintf("%v.%v%v", strings.TrimSuffix(filename, path.Ext(filename)), hash(), path.Ext(filename)), "-")
		},
		"extensions": func() string { return strings.TrimPrefix(path.Ext(filename), ".") },
	}
}

//	'GetURL' get default URL for a model based on its options
func (b Base) GetURL(option *Option, scope *gorm.Scope, field *gorm.Field, templater URLTemplater) string {
	if path := templater.GetURLTemplate(option); path != "" {
		tmpl := template.New("").Funcs(getFuncMap(scope, field, b.GetFileName()))
		if tmpl, err := tmpl.Parse(path); err == nil {
			var result = bytes.NewBufferString("")
			if err := tmpl.Execute(result, scope.Value); err == nil {
				return result.String()
			}
		}
	}
	return ""
}

//	'Cropped' mark the image to be cropped
func (b *Base) Cropped(values ...bool) (result bool) {
	result = b.cropped
	for _, value := range values {
		b.cropped = value
	}
	return result
}

//	'NeedCrop' return the file needs to be cropped or not
func (b *Base) NeedCrop() bool {
	return b.Crop
}

//	'GetCropOption' get crop options
func (b *Base) GetCropOption(name string) *image.Rectangle {
	if cropOption := b.CropOptions[strings.Split(name, "@")[0]]; cropOption != nil {
		return &image.Rectangle{
			Min: image.Point{X: cropOption.X, Y: cropOption.Y},
			Max: image.Point{X: cropOption.X + cropOption.Width, Y: cropOption.Y + cropOption.Height},
		}
	}
	return nil
}

//	'Retrieve' retrieve file content with url
func (b Base) Retrieve(url string) (*os.File, error) {
	return nil, errors.New("not implemented")
}

//	'GetSizes' get configured sizes, it will be used to crop images accordingly
func (b Base) GetSizes() map[string]*Size {
	return map[string]*Size{}
}

//	'IsImage' return if it is an image
func (b Base) IsImage() bool {
	_, err := getImageFormat(b.URL())
	return err == nil
}

func (b Base) IsVideo() bool {
	return isVideoFormat(b.URL())
}

func init() {
	admin.RegisterViewPath("github.com/Sky-And-Hammer/media_library/views")
}

//	'ConfigureECMetaBeforeInitialize' configure this field for ec admin
func (Base) ConfigureECMetaBeforeInitialize(meta resource.Metaor) {
	if meta, ok := meta.(*admin.Meta); ok {
		meta.Type = "file"
		if meta.GetFormattedValuer() == nil {
			meta.SetFormattedValuer(func(value interface{}, context *TM_EC.Context) interface{} {
				return utils.Stringify(meta.GetValuer()(value, context))
			})
		}
	}
}

func getImageFormat(url string) (*imaging.Format, error) {
	formats := map[string]imaging.Format{
		".jpg":  imaging.JPEG,
		".jpeg": imaging.JPEG,
		".png":  imaging.PNG,
		".tif":  imaging.TIFF,
		".tiff": imaging.TIFF,
		".bmp":  imaging.BMP,
		".gif":  imaging.GIF,
	}

	ext := strings.ToLower(regexp.MustCompile(`(\?.*?$)`).ReplaceAllString(filepath.Ext(url), ""))
	if f, ok := formats[ext]; ok {
		return &f, nil
	}
	return nil, imaging.ErrUnsupportedFormat
}

func isVideoFormat(name string) bool {
	formats := []string{".mp4", ".m4p", ".m4v", ".m4v", ".mov", ".mpeg", ".webm", ".avi", ".ogg", ".ogv"}
	ext := strings.ToLower(regexp.MustCompile(`(\?,*?$)`).ReplaceAllString(filepath.Ext(name), ""))
	for _, format := range formats {
		if format == ext {
			return true
		}
	}
	return false
}
