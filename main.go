package media_library

import (
	"database/sql/driver"
	"image"
	"io"
	"os"
	"strings"

	//  The fantastic ORM library for Golang, aims to be developer friendly.
	"github.com/jinzhu/gorm"

	"github.com/Sky-And-Hammer/TM_EC/utils"
)

// 'Size' is a struct, used for 'GetSizes' method, is will return a slice of size, media library will crop images automatically based on it
type Size struct {
	Width, Height int
}

// 'URLTemplater' is a interface to return url template
type URLTemplater interface {
	GetURLTemplate(*Option) string
}

// 'Media' is an interface including methods that needs for a media library storages
type Media interface {
	Scan(value interface{}) error
	Value() (driver.Value, error)

	GetURLTemplate(*Option) string
	GetURL(option *Option, scope *gorm.Scope, field *gorm.Field, templater URLTemplater) string

	GetFileHeader() FileHeader
	GetFileName() string

	GetSizes() map[string]*Size
	NeedCrop() bool
	Cropped(values ...bool) bool
	GetCropOption(name string) *image.Rectangle

	Store(url string, option *Option, reader io.Reader) error
	Retrieve(url string) (*os.File, error)

	IsImage() bool

	URL(style ...string) string
	Ext() string
	String() string
}

// 'Option' media library option
type Option map[string]string

// 'Get' used to get option with name
func (option Option) Get(key string) string {
	return option[strings.ToUpper(key)]
}

func parseTagOption(str string) *Option {
	option := Option(utils.ParseTagOption(str))
	return &option
}