package pools

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

func ReadCustomValidatorsFile(validatorKeysFile string) (validatorKeys [][]byte, err error) {
	log.Info("Reading validator keys from .txt: ", validatorKeysFile)
	validatorKeys = make([][]byte, 0)

	file, err := os.Open(validatorKeysFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip first line
		if (line == "f_validator_pubkey") || (line == "f0_") || (line == "f_public_key") {
			continue
		}
		keyStr := strings.Trim(line, "\"")
		if strings.Contains(keyStr, "\\x") {
			keyStr = strings.Replace(line, "\\x", "", -1)
		}
		if !strings.HasPrefix(keyStr, "0x") {
			keyStr = "0x" + keyStr
		}

		if len(keyStr) != 98 {
			return validatorKeys, errors.New(fmt.Sprintf("length of key is incorrect: %d", len(keyStr)))
		}

		valKey, err := hexutil.Decode(keyStr)
		if err != nil {
			return validatorKeys, errors.Wrap(err, fmt.Sprintf("could not decode key: %s", keyStr))
		}
		validatorKeys = append(validatorKeys, valKey)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	log.Info("Done reading ", len(validatorKeys), " from ", validatorKeysFile)
	return validatorKeys, nil
}

func ReadEthstaValidatorsFile(validatorKeysFile string) (validatorKeys [][]byte, err error) {
	log.Info("Reading validator keys from ethsta.com csv file: ", validatorKeysFile)
	validatorKeys = make([][]byte, 0)

	file, err := os.Open(validatorKeysFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		// Skip first line
		line := scanner.Text()
		if line == "address,version,entity" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) != 3 {
			return validatorKeys, errors.New("the format of the file is not the expected, see ethsta.com")
		}
		keyStr := "0x" + fields[0]

		if len(keyStr) != 98 {
			return validatorKeys, errors.New(fmt.Sprintf("length of key is incorrect: %d", len(keyStr)))
		}
		valKey, err := hexutil.Decode(keyStr)
		if err != nil {
			return validatorKeys, errors.Wrap(err, fmt.Sprintf("could not decode key: %s", keyStr))
		}
		validatorKeys = append(validatorKeys, valKey)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	log.Info("Done reading ", len(validatorKeys), " from ", validatorKeysFile)
	return validatorKeys, nil
}
