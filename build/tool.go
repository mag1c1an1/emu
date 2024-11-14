package build

import (
	"fmt"
	"runtime"
	"strings"
)

func GenerateBatchByIpTable(nodeNum, shardNum int) error {
	// read IP table file first
	ipMap := readIpTable("./ipTable.json")

	// determine the formats of commands and fileNames, according to operating system
	var fileNameFormat, commandFormat string
	os := runtime.GOOS
	switch os {
	case "windows":
		fileNameFormat = "compile_run_IpAddr=%s.bat"
		commandFormat = "start cmd /k go run main.go"
	default:
		fileNameFormat = "compile_run_IpAddr=%s.sh"
		commandFormat = "go run main.go"
	}

	// generate file for each ip
	for i := 0; i < shardNum; i++ {
		// if this shard is not existed, return
		if _, shardExist := ipMap[uint64(i)]; !shardExist {
			return fmt.Errorf("the shard (shardID = %d) is not existed in the IP Table file", i)
		}
		// if this shard is existed.
		for j := 0; j < nodeNum; j++ {
			if nodeIp, nodeExist := ipMap[uint64(i)][uint64(j)]; nodeExist {
				// attach this command to this file
				ipAddr := strings.Split(nodeIp, ":")[0]
				batFilePath := fmt.Sprintf(fileNameFormat, strings.ReplaceAll(ipAddr, ".", "_"))
				command := fmt.Sprintf(commandFormat+" -n %d -N %d -s %d -S %d\n", j, nodeNum, i, shardNum)
				if err := attachLineToFile(batFilePath, command); nil != err {
					return err
				}
			} else {
				return fmt.Errorf("the node (shardID = %d, nodeID = %d) is not existed in the IP Table file", i, j)
			}
		}
	}

	// generate command for supervisor
	if supervisorShard, shardExist := ipMap[2147483647]; shardExist {
		if nodeIp, nodeExist := supervisorShard[0]; nodeExist {
			ipAddr := strings.Split(nodeIp, ":")[0]
			batFilePath := fmt.Sprintf(fileNameFormat, strings.ReplaceAll(ipAddr, ".", "_"))
			supervisorCommand := fmt.Sprintf(commandFormat+" -c -N %d -S %d\n", nodeNum, shardNum)
			if err := attachLineToFile(batFilePath, supervisorCommand); nil != err {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("the supervisor (shardID = 2147483647, nodeID = 0) is not existed in the IP Table file")
}

func GenerateExeBatchByIpTable(nodeNum, shardNum int) error {
	// read IP table file first
	ipMap := readIpTable("./ipTable.json")

	// determine the formats of commands and fileNames, according to operating system
	var fileNameFormat, commandFormat string
	os := runtime.GOOS
	switch os {
	case "windows":
		fileNameFormat = os + "_exe_run_IpAddr=%s.bat"
		commandFormat = "start cmd /k blockEmulator_Windows_Precompile.exe"
	default:
		fileNameFormat = "run_IpAddr=%s.sh"
		commandFormat = "./emu"
	}

	// generate file for each ip
	for i := 0; i < shardNum; i++ {
		// if this shard is not existed, return
		if _, shardExist := ipMap[uint64(i)]; !shardExist {
			return fmt.Errorf("the shard (shardID = %d) is not existed in the IP Table file", i)
		}
		// if this shard is existed.
		for j := 0; j < nodeNum; j++ {
			if nodeIp, nodeExist := ipMap[uint64(i)][uint64(j)]; nodeExist {
				// attach this command to this file
				ipAddr := strings.Split(nodeIp, ":")[0]
				batFilePath := fmt.Sprintf(fileNameFormat, strings.ReplaceAll(ipAddr, ".", "_"))
				command := fmt.Sprintf(commandFormat+" -n %d -N %d -s %d -S %d\n", j, nodeNum, i, shardNum)
				if err := attachLineToFile(batFilePath, command); nil != err {
					return err
				}
			} else {
				return fmt.Errorf("the node (shardID = %d, nodeID = %d) is not existed in the IP Table file", i, j)
			}
		}
	}

	// generate command for supervisor
	if supervisorShard, shardExist := ipMap[2147483647]; shardExist {
		if nodeIp, nodeExist := supervisorShard[0]; nodeExist {
			ipAddr := strings.Split(nodeIp, ":")[0]
			batFilePath := fmt.Sprintf(fileNameFormat, strings.ReplaceAll(ipAddr, ".", "_"))
			supervisorCommand := fmt.Sprintf(commandFormat+" -c -N %d -S %d\n", nodeNum, shardNum)
			if err := attachLineToFile(batFilePath, supervisorCommand); nil != err {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("the supervisor (shardID = 2147483647, nodeID = 0) is not existed in the IP Table file")
}
