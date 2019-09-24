package resourcemgr

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseAllMemoryString(allocatableMem, allocatedMem string) (total, allocated, free uint32, err error) {
	if strings.HasSuffix(allocatableMem, "Ki") {
		var totalMemInt int
		// Cut last two chars, convert to int
		totalMemInt, err = strconv.Atoi(allocatableMem[0:(len(allocatableMem) - 2)])
		if err != nil {
			return
		}

		// Calculate total memory (in megabytes)
		total = uint32(totalMemInt / 1024)
	}

	allocated, err = ParseMemoryString(allocatedMem)
	if err != nil {
		return
	}

	// Calculate free space
	free = total - allocated

	return
}

func ParseMemoryString(memory string) (mem uint32, err error) {
	var memoryInt int
	// In case the value represented in Megabytes and "Mi" is the suffix
	if strings.HasSuffix(memory, "Mi") {
		// Cut last two chars
		memoryInt, err = strconv.Atoi(memory[0:(len(memory) - 2)])
		if err != nil {
			return
		}
	} else {
		memoryInt, err = strconv.Atoi(memory)
		if err != nil {
			return
		}
	}

	return uint32(memoryInt), nil
}

func ParseCpuString(cpuStr string) (cpu float64, err error) {
	// In case the value represented in milli cores and "m" is the suffix
	if strings.HasSuffix(cpuStr, "m") {
		// Cut last char, convert to int
		cpu, err = strconv.ParseFloat(strings.Replace(cpuStr, "m", "", -1), 64)
		if nil != err {
			return
		}
	} else {
		cpu, err = strconv.ParseFloat(cpuStr, 64)
		if err != nil {
			return
		}
	}

	return
}

func ValidateInputs(cpu, defaultCpu float64, memory, defaultMem uint32) error {

	if cpu > 0 && cpu < defaultCpu {
		return fmt.Errorf("requested CPU limit value: %f.The value must be greater than %f", cpu, defaultCpu)
	}

	if memory > 0 && memory < defaultMem {
		return fmt.Errorf("requested memory value: %d.The value must be greater than %d", memory, defaultMem)
	}

	return nil
}
