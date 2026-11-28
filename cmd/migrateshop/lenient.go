package main

import (
	"reflect"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/bsonrw"
)

// ShoxPos Mongo'si sxemasiz: bir maydonda turli turlar uchraydi (masalan
// shopCategories ichida son, narx matn sifatida). Ko'chirish bitta "iflos"
// hujjat tufayli to'xtamasligi uchun asosiy turlar YUMSHOQ o'qiladi: mos
// kelmasa — nol qiymat (keyingi tekshiruvlar uni o'tkazib yuboradi va
// hisobotda ko'rsatadi). Struct'lar oddiy Go turlarida qoladi.

func lenientRegistry() *bsoncodec.Registry {
	reg := bson.NewRegistry()
	reg.RegisterTypeDecoder(reflect.TypeOf(""), bsoncodec.ValueDecoderFunc(decString))
	reg.RegisterTypeDecoder(reflect.TypeOf(float64(0)), bsoncodec.ValueDecoderFunc(decFloat))
	reg.RegisterTypeDecoder(reflect.TypeOf(0), bsoncodec.ValueDecoderFunc(decInt))
	reg.RegisterTypeDecoder(reflect.TypeOf(false), bsoncodec.ValueDecoderFunc(decBool))
	reg.RegisterTypeDecoder(reflect.TypeOf(time.Time{}), bsoncodec.ValueDecoderFunc(decTime))
	return reg
}

var (
	rawValueType  = reflect.TypeOf(bson.RawValue{})
	plainRegistry = bson.NewRegistry()
)

// decodeLenient — bitta hujjatni yumshoq registry bilan o'qiydi.
func decodeLenient(raw bson.Raw, v any) error {
	d, err := bson.NewDecoder(bsonrw.NewBSONDocumentReader(raw))
	if err != nil {
		return err
	}
	if err := d.SetRegistry(lenientRegistry()); err != nil {
		return err
	}
	return d.Decode(v)
}

func readRaw(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader) (bson.RawValue, error) {
	var rv bson.RawValue
	dec, err := plainRegistry.LookupDecoder(rawValueType)
	if err != nil {
		return rv, err
	}
	err = dec.DecodeValue(dc, vr, reflect.ValueOf(&rv).Elem())
	return rv, err
}

func asString(v bson.RawValue) string {
	switch v.Type {
	case bson.TypeString:
		return v.StringValue()
	case bson.TypeInt32:
		return strconv.FormatInt(int64(v.Int32()), 10)
	case bson.TypeInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case bson.TypeDouble:
		return strconv.FormatFloat(v.Double(), 'f', -1, 64)
	case bson.TypeObjectID:
		return v.ObjectID().Hex()
	}
	return ""
}

func asFloat(v bson.RawValue) float64 {
	switch v.Type {
	case bson.TypeDouble:
		return v.Double()
	case bson.TypeInt32:
		return float64(v.Int32())
	case bson.TypeInt64:
		return float64(v.Int64())
	case bson.TypeDecimal128:
		f, _ := strconv.ParseFloat(v.Decimal128().String(), 64)
		return f
	case bson.TypeString:
		f, _ := strconv.ParseFloat(strings.NewReplacer(" ", "", ",", "").Replace(v.StringValue()), 64)
		return f
	case bson.TypeBoolean:
		if v.Boolean() {
			return 1
		}
	}
	return 0
}

func decString(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	v, err := readRaw(dc, vr)
	if err != nil {
		return err
	}
	val.SetString(asString(v))
	return nil
}

func decFloat(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	v, err := readRaw(dc, vr)
	if err != nil {
		return err
	}
	val.SetFloat(asFloat(v))
	return nil
}

func decInt(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	v, err := readRaw(dc, vr)
	if err != nil {
		return err
	}
	val.SetInt(int64(asFloat(v)))
	return nil
}

func decBool(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	v, err := readRaw(dc, vr)
	if err != nil {
		return err
	}
	switch v.Type {
	case bson.TypeBoolean:
		val.SetBool(v.Boolean())
	case bson.TypeString:
		val.SetBool(strings.EqualFold(v.StringValue(), "true"))
	default:
		val.SetBool(asFloat(v) != 0)
	}
	return nil
}

func decTime(dc bsoncodec.DecodeContext, vr bsonrw.ValueReader, val reflect.Value) error {
	v, err := readRaw(dc, vr)
	if err != nil {
		return err
	}
	var t time.Time
	switch v.Type {
	case bson.TypeDateTime:
		t = v.Time()
	case bson.TypeTimestamp:
		sec, _ := v.Timestamp()
		t = time.Unix(int64(sec), 0)
	case bson.TypeString:
		if p, err := time.Parse(time.RFC3339Nano, v.StringValue()); err == nil {
			t = p
		}
	case bson.TypeInt64:
		t = time.UnixMilli(v.Int64())
	}
	val.Set(reflect.ValueOf(t))
	return nil
}
