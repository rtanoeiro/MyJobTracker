package utils

import (
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func ConvertFromStrToInt32(value string) (int32, error) {
	valueInt, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(valueInt), nil
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func ConvertFromStrToPGTypeDate(date string) pgtype.Date {
	dateParsed, _ := time.Parse("2006-01-02", date)
	return pgtype.Date{Time: dateParsed, Valid: date != ""}
}

func PGText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}
