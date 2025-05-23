// Code generated by go-enum DO NOT EDIT.
// Version:
// Revision:
// Build Date:
// Built By:

package favorite

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
)

const (
	// ListTypePersonal is a ListType of type Personal.
	ListTypePersonal ListType = iota + 1
	// ListTypeGroup is a ListType of type Group.
	ListTypeGroup
)

var ErrInvalidListType = fmt.Errorf("not a valid ListType, try [%s]", strings.Join(_ListTypeNames, ", "))

const _ListTypeName = "personalgroup"

var _ListTypeNames = []string{
	_ListTypeName[0:8],
	_ListTypeName[8:13],
}

// ListTypeNames returns a list of possible string values of ListType.
func ListTypeNames() []string {
	tmp := make([]string, len(_ListTypeNames))
	copy(tmp, _ListTypeNames)
	return tmp
}

// ListTypeValues returns a list of the values for ListType
func ListTypeValues() []ListType {
	return []ListType{
		ListTypePersonal,
		ListTypeGroup,
	}
}

var _ListTypeMap = map[ListType]string{
	ListTypePersonal: _ListTypeName[0:8],
	ListTypeGroup:    _ListTypeName[8:13],
}

// String implements the Stringer interface.
func (x ListType) String() string {
	if str, ok := _ListTypeMap[x]; ok {
		return str
	}
	return fmt.Sprintf("ListType(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x ListType) IsValid() bool {
	_, ok := _ListTypeMap[x]
	return ok
}

var _ListTypeValue = map[string]ListType{
	_ListTypeName[0:8]:  ListTypePersonal,
	_ListTypeName[8:13]: ListTypeGroup,
}

// ParseListType attempts to convert a string to a ListType.
func ParseListType(name string) (ListType, error) {
	if x, ok := _ListTypeValue[name]; ok {
		return x, nil
	}
	return ListType(0), fmt.Errorf("%s is %w", name, ErrInvalidListType)
}

// MarshalText implements the text marshaller method.
func (x ListType) MarshalText() ([]byte, error) {
	return []byte(x.String()), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *ListType) UnmarshalText(text []byte) error {
	name := string(text)
	tmp, err := ParseListType(name)
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}

var errListTypeNilPtr = errors.New("value pointer is nil") // one per type for package clashes

// Scan implements the Scanner interface.
func (x *ListType) Scan(value interface{}) (err error) {
	if value == nil {
		*x = ListType(0)
		return
	}

	// A wider range of scannable types.
	// driver.Value values at the top of the list for expediency
	switch v := value.(type) {
	case int64:
		*x = ListType(v)
	case string:
		*x, err = ParseListType(v)
	case []byte:
		*x, err = ParseListType(string(v))
	case ListType:
		*x = v
	case int:
		*x = ListType(v)
	case *ListType:
		if v == nil {
			return errListTypeNilPtr
		}
		*x = *v
	case uint:
		*x = ListType(v)
	case uint64:
		*x = ListType(v)
	case *int:
		if v == nil {
			return errListTypeNilPtr
		}
		*x = ListType(*v)
	case *int64:
		if v == nil {
			return errListTypeNilPtr
		}
		*x = ListType(*v)
	case float64: // json marshals everything as a float64 if it's a number
		*x = ListType(v)
	case *float64: // json marshals everything as a float64 if it's a number
		if v == nil {
			return errListTypeNilPtr
		}
		*x = ListType(*v)
	case *uint:
		if v == nil {
			return errListTypeNilPtr
		}
		*x = ListType(*v)
	case *uint64:
		if v == nil {
			return errListTypeNilPtr
		}
		*x = ListType(*v)
	case *string:
		if v == nil {
			return errListTypeNilPtr
		}
		*x, err = ParseListType(*v)
	}

	return
}

// Value implements the driver Valuer interface.
func (x ListType) Value() (driver.Value, error) {
	return x.String(), nil
}

const (
	// MemberTypeOwner is a MemberType of type Owner.
	MemberTypeOwner MemberType = iota + 1
	// MemberTypeAdmin is a MemberType of type Admin.
	MemberTypeAdmin
	// MemberTypeEditor is a MemberType of type Editor.
	MemberTypeEditor
	// MemberTypeViewer is a MemberType of type Viewer.
	MemberTypeViewer
)

var ErrInvalidMemberType = fmt.Errorf("not a valid MemberType, try [%s]", strings.Join(_MemberTypeNames, ", "))

const _MemberTypeName = "owneradmineditorviewer"

var _MemberTypeNames = []string{
	_MemberTypeName[0:5],
	_MemberTypeName[5:10],
	_MemberTypeName[10:16],
	_MemberTypeName[16:22],
}

// MemberTypeNames returns a list of possible string values of MemberType.
func MemberTypeNames() []string {
	tmp := make([]string, len(_MemberTypeNames))
	copy(tmp, _MemberTypeNames)
	return tmp
}

// MemberTypeValues returns a list of the values for MemberType
func MemberTypeValues() []MemberType {
	return []MemberType{
		MemberTypeOwner,
		MemberTypeAdmin,
		MemberTypeEditor,
		MemberTypeViewer,
	}
}

var _MemberTypeMap = map[MemberType]string{
	MemberTypeOwner:  _MemberTypeName[0:5],
	MemberTypeAdmin:  _MemberTypeName[5:10],
	MemberTypeEditor: _MemberTypeName[10:16],
	MemberTypeViewer: _MemberTypeName[16:22],
}

// String implements the Stringer interface.
func (x MemberType) String() string {
	if str, ok := _MemberTypeMap[x]; ok {
		return str
	}
	return fmt.Sprintf("MemberType(%d)", x)
}

// IsValid provides a quick way to determine if the typed value is
// part of the allowed enumerated values
func (x MemberType) IsValid() bool {
	_, ok := _MemberTypeMap[x]
	return ok
}

var _MemberTypeValue = map[string]MemberType{
	_MemberTypeName[0:5]:   MemberTypeOwner,
	_MemberTypeName[5:10]:  MemberTypeAdmin,
	_MemberTypeName[10:16]: MemberTypeEditor,
	_MemberTypeName[16:22]: MemberTypeViewer,
}

// ParseMemberType attempts to convert a string to a MemberType.
func ParseMemberType(name string) (MemberType, error) {
	if x, ok := _MemberTypeValue[name]; ok {
		return x, nil
	}
	return MemberType(0), fmt.Errorf("%s is %w", name, ErrInvalidMemberType)
}

// MarshalText implements the text marshaller method.
func (x MemberType) MarshalText() ([]byte, error) {
	return []byte(x.String()), nil
}

// UnmarshalText implements the text unmarshaller method.
func (x *MemberType) UnmarshalText(text []byte) error {
	name := string(text)
	tmp, err := ParseMemberType(name)
	if err != nil {
		return err
	}
	*x = tmp
	return nil
}

var errMemberTypeNilPtr = errors.New("value pointer is nil") // one per type for package clashes

// Scan implements the Scanner interface.
func (x *MemberType) Scan(value interface{}) (err error) {
	if value == nil {
		*x = MemberType(0)
		return
	}

	// A wider range of scannable types.
	// driver.Value values at the top of the list for expediency
	switch v := value.(type) {
	case int64:
		*x = MemberType(v)
	case string:
		*x, err = ParseMemberType(v)
	case []byte:
		*x, err = ParseMemberType(string(v))
	case MemberType:
		*x = v
	case int:
		*x = MemberType(v)
	case *MemberType:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = *v
	case uint:
		*x = MemberType(v)
	case uint64:
		*x = MemberType(v)
	case *int:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = MemberType(*v)
	case *int64:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = MemberType(*v)
	case float64: // json marshals everything as a float64 if it's a number
		*x = MemberType(v)
	case *float64: // json marshals everything as a float64 if it's a number
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = MemberType(*v)
	case *uint:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = MemberType(*v)
	case *uint64:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x = MemberType(*v)
	case *string:
		if v == nil {
			return errMemberTypeNilPtr
		}
		*x, err = ParseMemberType(*v)
	}

	return
}

// Value implements the driver Valuer interface.
func (x MemberType) Value() (driver.Value, error) {
	return x.String(), nil
}
