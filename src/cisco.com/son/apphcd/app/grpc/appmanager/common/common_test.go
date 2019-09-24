package common

import (
	"reflect"
	"testing"

	"cisco.com/son/apphcd/api/v1/appmanager"
)

func TestPeriodicToCronString(t *testing.T) {
	req := &appmanager.CreateAppRequest{}
	req.CyclePeriodicAttr = &appmanager.CyclePeriodicReqAttr{}
	req.CyclePeriodicAttr.MaxStartHour = 1
	req.CyclePeriodicAttr.MinStartHour = 23

    t.Log(PeriodicToCronString(req))
}

func TestCronStringToCyclePeriodicRespAttr(t *testing.T) {

	cronString := "*/1 3-4 * * 0"
	cronObjOrig := &appmanager.CyclePeriodicRespAttr{
		IntervalMin:  0,
		MinStartHour: 3,
		MaxStartHour: 4,
		WorkingDays:  []string{WeekDaySunday},
	}

	cronObjNew ,err := CronStringToCyclePeriodicRespAttr(cronString)
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(cronObjOrig, cronObjNew) {
		t.Fatal("equal objects")
	}

	cronString = "*/10 0-22 * * 0,1,2,3,4,5,6"
	cronObjOrig = &appmanager.CyclePeriodicRespAttr{
		IntervalMin: 10,
		MinStartHour: 0,
		MaxStartHour: 22,
		WorkingDays:[]string{WeekDaySunday,WeekDayMonday,WeekDayTuesday,WeekDayWednesday,WeekDayThursday,WeekDayFriday,WeekDaySaturday},
	}

	cronObjNew, err = CronStringToCyclePeriodicRespAttr(cronString)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(cronObjOrig, cronObjNew) {
		t.Fatal("not equal ob jects")
	}


	cronString = "*/1 23,0,1,2,3,4,5,6 * * 0"
	cronObjOrig = &appmanager.CyclePeriodicRespAttr{
		IntervalMin:  1,
		MinStartHour: 23,
		MaxStartHour: 6,
		WorkingDays:  []string{WeekDaySunday},
	}

	cronObjNew ,err = CronStringToCyclePeriodicRespAttr(cronString)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(cronObjOrig, cronObjNew) {
		t.Fatal("equal objects")
	}

}

func TestValidateDockerImage(t *testing.T) {

	if err := ValidateDockerImage("maayanfriedman/flexdemo2", "1.0.0"); err != nil {
		t.Fatal(err)
	}
}




