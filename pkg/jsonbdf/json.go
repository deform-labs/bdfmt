package jsonbdf

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

// ToJSON converts a BDF Tag tree to pretty-printed or compact JSON
func ToJSON(t bdf.Tag, pretty bool) ([]byte, error) {
	val := tagToInterface(t)
	if pretty {
		return json.MarshalIndent(val, "", "  ")
	}
	return json.Marshal(val)
}

func tagToInterface(t bdf.Tag) interface{} {
	switch v := t.(type) {
	case bdf.TagByte:
		return int8(v)
	case bdf.TagShort:
		return int16(v)
	case bdf.TagInt:
		return int32(v)
	case bdf.TagLong:
		return int64(v)
	case bdf.TagFloat:
		return float32(v)
	case bdf.TagDouble:
		return float64(v)
	case bdf.TagString:
		return string(v)
	case bdf.TagByteArray:
		res := make([]int, len(v))
		for i, b := range v {
			res[i] = int(int8(b))
		}
		return res
	case bdf.TagIntArray:
		res := make([]int32, len(v))
		copy(res, v)
		return res
	case bdf.TagLongArray:
		res := make([]int64, len(v))
		copy(res, v)
		return res
	case *bdf.TagList:
		res := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			res[i] = tagToInterface(el)
		}
		return res
	case *bdf.TagCompound:
		m := make(map[string]interface{}, v.Len())
		for _, e := range v.Entries() {
			m[e.Key] = tagToInterface(e.Value)
		}
		return m
	default:
		return nil
	}
}

// FromJSON parses JSON data into a BDF Tag
func FromJSON(data []byte) (bdf.Tag, error) {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return interfaceToTag(raw)
}

func interfaceToTag(val interface{}) (bdf.Tag, error) {
	switch v := val.(type) {
	case bool:
		if v {
			return bdf.TagByte(1), nil
		}
		return bdf.TagByte(0), nil
	case float64:
		// Check if it's an integer
		if math.Floor(v) == v {
			if v >= math.MinInt32 && v <= math.MaxInt32 {
				return bdf.TagInt(int32(v)), nil
			}
			return bdf.TagLong(int64(v)), nil
		}
		return bdf.TagDouble(v), nil
	case string:
		return bdf.TagString(v), nil
	case []interface{}:
		if len(v) == 0 {
			return &bdf.TagList{ElementType: bdf.TagTypeEnd, Elements: []bdf.Tag{}}, nil
		}
		elements := make([]bdf.Tag, len(v))
		var elemType bdf.TagType = bdf.TagTypeEnd
		for i, item := range v {
			tag, err := interfaceToTag(item)
			if err != nil {
				return nil, err
			}
			elements[i] = tag
			if i == 0 {
				elemType = tag.Type()
			}
		}
		return &bdf.TagList{ElementType: elemType, Elements: elements}, nil
	case map[string]interface{}:
		comp := bdf.NewTagCompound()
		for k, item := range v {
			tag, err := interfaceToTag(item)
			if err != nil {
				return nil, err
			}
			comp.Set(k, tag)
		}
		return comp, nil
	default:
		return nil, fmt.Errorf("unsupported JSON type: %T", val)
	}
}
