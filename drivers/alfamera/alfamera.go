package alfamera

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"

	//"qBox/drivers/alfamera"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"

	"github.com/aldas/go-modbus-client"
	"github.com/npat-efault/crc16"
)

type Alfamera struct {
	logger  *log.LoggerService
	number  byte
	network *net.Network
}

func (alfamera *Alfamera) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	alfamera.logger = logger
	alfamera.network = network
	alfamera.number = counterNumber

	alfamera.logger.Info("Инициализация прибора,№ %d", alfamera.number)

	b := modbus.NewRequestBuilder("10.178.4.14:951", 1)
	requests, err := b.
		Add(b.Byte(0xEF04, true)).
		Add(b.Byte(0xEF05, false)).ReadHoldingRegistersRTU()

	if err != nil {
		return err
	}

	client := modbus.NewRTUClient()
	defer client.Close()
	if err := client.Connect(context.Background(), "10.178.4.14:951"); err != nil {
		return err
	}

	for _, req := range requests {
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			return err
		}
		_, err = req.ExtractFields(resp.(modbus.RegistersResponse), true)
		if err != nil {
			return err
		}
	}

	return nil
}

func (alfamera *Alfamera) Read(*models.DataDevice, error) {

	// var response [] byte
	// var err error

	// alfamera.logger.Info("_")
	// response, err = alfamera.runIO([]byte{})

}

func (alfamera *Alfamera) checkResponse(response []byte) bool {

	if len(response) < 3 {
		alfamera.logger.Info("")
		return false
	}

	if response[0] != alfamera.number || response[1] != 0000 {
		alfamera.logger.Info("")
		return false
	}

	calculatedCheckSum := intToLittleEndian(crc16.Checksum(crc16.Modbus, response[:len(response)-2]))
	checkSumResponse := response[len(response)-2:]
	if calculatedCheckSum[0] != checkSumResponse[0] || calculatedCheckSum[1] != checkSumResponse[1] {
		alfamera.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		alfamera.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		alfamera.logger.Debug("Получена контрольная сумма- %X", checkSumResponse)
		return false
	}

	return true
}

func intToLittleEndian(i uint16) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, i)
	return buf.Bytes()
}

func main() {

	b := modbus.NewRequestBuilder("10.178.4.14:951", 1)
	//Трубопровод 1
	requests, _ := b.
		Add(b.Byte(0x1600, true)).
		Add(b.Byte(0x1600, false)).
		Add(b.Byte(0x1601, true)).
		Add(b.Byte(0x1601, false)).
		Add(b.Byte(0x1602, true)).
		Add(b.Byte(0x1602, false)).
		Add(b.Byte(0x1603, true)).
		Add(b.Byte(0x1603, false)).
		Add(b.Byte(0x1604, true)).
		Add(b.Byte(0x1604, false)).
		Add(b.Byte(0x1605, true)).
		Add(b.Byte(0x1605, false)).
		Add(b.Byte(0x1606, true)).
		Add(b.Byte(0x1606, false)).
		Add(b.Byte(0x1607, true)).
		Add(b.Byte(0x1607, false)).
		Add(b.Byte(0x1608, true)).
		Add(b.Byte(0x1608, false)).
		Add(b.Byte(0x1609, true)).
		Add(b.Byte(0x1609, false)).
		Add(b.Byte(0x1610, true)).
		Add(b.Byte(0x1610, false)).
		Add(b.Byte(0x1611, true)).
		Add(b.Byte(0x1611, false)).
		Add(b.Byte(0x1612, true)).
		Add(b.Byte(0x1612, false)).
		Add(b.Byte(0x1613, true)).
		Add(b.Byte(0x1613, false)).
		ReadHoldingRegistersRTU()

	client := modbus.NewRTUClient()
	defer client.Close()
	if err := client.Connect(context.Background(), "10.178.4.14:951"); err != nil {
		panic(err)
	}

	for _, req := range requests {
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			panic(err)
		}

		fmt.Println(resp)

		// registers, _ := resp.(*packet.ReadHoldingRegistersResponseRTU).AsRegisters(req.StartAddress)
		// alarmDo1, _ := registers.Byte(0x1600, true)
		// // alarmDo2, _ := registers.Byte(0x1600, false)
		// alarmDo3, _ := registers.Byte(0x1601, true)
		// alarmDo4, _ := registers.Byte(0x1601, false)
		// fmt.Printf("%+v\n", alarmDo1)
		//Конвертация трубопровода 1
		fields, _ := req.ExtractFields(resp.(modbus.RegistersResponse), true)
		fmt.Printf("Массовый расход ( qm ): %.2f кг/ч\n", math.Float32frombits(ToLong([4]byte{fields[2].Value.(byte), fields[3].Value.(byte), fields[0].Value.(byte), fields[1].Value.(byte)})))
		fmt.Printf("Кол-во теплоты ( W ): %.2f Гкал/ч\n", 0.000000238843*math.Float32frombits(ToLong([4]byte{fields[6].Value.(byte), fields[7].Value.(byte), fields[4].Value.(byte), fields[5].Value.(byte)})))
		fmt.Printf("Давление ( P ): %.2f кПа\n", math.Float32frombits(ToLong([4]byte{fields[10].Value.(byte), fields[11].Value.(byte), fields[8].Value.(byte), fields[9].Value.(byte)})))
		fmt.Printf("Температура ( T ): %.2f °C\n", math.Float32frombits(ToLong([4]byte{fields[14].Value.(byte), fields[15].Value.(byte), fields[12].Value.(byte), fields[13].Value.(byte)})))
		fmt.Printf("Перепады давления ( dP ): %.2f кПа\n", math.Float32frombits(ToLong([4]byte{fields[18].Value.(byte), fields[19].Value.(byte), fields[16].Value.(byte), fields[17].Value.(byte)})))
		fmt.Printf("Энтальпия ( h ): %.2f кКал/кг\n", math.Float32frombits(ToLong([4]byte{fields[22].Value.(byte), fields[23].Value.(byte), fields[20].Value.(byte), fields[21].Value.(byte)})))
		fmt.Printf("Плотность ( r ): %.2f кг/м3\n", math.Float32frombits(ToLong([4]byte{fields[26].Value.(byte), fields[27].Value.(byte), fields[24].Value.(byte), fields[25].Value.(byte)})))
	}
}

func ToLong(bytes [4]byte) uint32 {
	var amount uint32 = 0
	for i := 0; i <= 3; i++ {
		amount += uint32(uint32(bytes[i]) << uint32(8*(3-i)))
	}
	return amount
}
