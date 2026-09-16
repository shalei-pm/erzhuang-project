package t1bi

import "testing"

func TestDecodeRowsMapsFrontDeskArrivals(t *testing.T) {
	rows, err := decodeRows(`{
		"code": 200,
		"msg": "ok",
		"data": {
			"data": [{
				"st_day": "2026-08-25",
				"visit_user_count": 55,
				"visit_front_desk": 3,
				"visit_no_consult": 34,
				"visit_need_consult": 18
			}]
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].VisitFrontDesk == nil || *rows[0].VisitFrontDesk != 3 {
		t.Fatalf("rows = %#v", rows)
	}
}
