package alfamera

import (
	"context"
	"fmt"
	"math"
	"qBox/models"
	"qBox/services/convert"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"

	"github.com/aldas/go-modbus-client"
)

type Alfamera struct {
	data    models.DataDevice
	logger  *log.LoggerService
	number  byte
	network *net.Network
	client  *modbus.Client
}

func (alfamera *Alfamera) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	alfamera.logger = logger
	alfamera.network = network
	alfamera.number = counterNumber

	alfamera.data.CoefficientKWh = 1 / 1.163 / 1000
	alfamera.data.CoefficientMWh = 1 / 1.163 // 0.85984522785
	alfamera.data.CoefficientGJ = 1 / (3.6 / alfamera.data.CoefficientMWh)

	alfamera.logger.Info("Инициализация прибора,№ %d", alfamera.number)

	b := modbus.NewRequestBuilder(fmt.Sprintf("%s:%d", network.GetIp(), network.GetPort()), alfamera.number)
	requests, err := b.
		Add(b.Byte(0xEF04, true)).
		Add(b.Byte(0xEF05, false)).ReadHoldingRegistersRTU()

	if err != nil {
		alfamera.logger.Error("%s", err)
		return err
	}

	client := modbus.NewRTUClient()
	defer client.Close()
	if err := client.Connect(context.Background(), fmt.Sprintf("%s:%d", network.GetIp(), network.GetPort())); err != nil {
		return err
	}

	for _, req := range requests {
		resp, err := client.Do(context.Background(), req)
		if err != nil {
			alfamera.logger.Error("%s", err)
			return err
		}
		_, err = req.ExtractFields(resp.(modbus.RegistersResponse), true)
		if err != nil {
			return err
		}
	}

	return nil
}

func (alfamera *Alfamera) Read() (*models.DataDevice, error) {
	//Инициализация client для работы с ModBus RTU
	client := modbus.NewRTUClient()
	defer client.Close()
	if err := client.Connect(context.Background(), fmt.Sprintf("%s:%d", alfamera.network.GetIp(), alfamera.network.GetPort())); err != nil {
		return &alfamera.data, err
	}
	//Подключение
	b1 := modbus.NewRequestBuilder(fmt.Sprintf("%s:%d", alfamera.network.GetIp(), alfamera.network.GetPort()), 1)
	b1 = alfamera.addTube(0, b1)
	// b = alfamera.timeToDate(b)
	// b = alfamera.serialToString(b)

	requests, _ := b1.ReadHoldingRegistersRTU()

	alfamera.data.TimeRequest = time.Now()
	alfamera.data.AddNewSystem(0)
	alfamera.data.Systems[0].Status = true

	resp, err := client.Do(context.Background(), requests[0])
	alfamera.logger.Debug("%s", resp)
	if err != nil {
		return &alfamera.data, err
	}
	fields, _ := requests[0].ExtractFields(resp.(modbus.RegistersResponse), true)
	alfamera.logger.Debug("%d", len(fields))
	alfamera.convertDataTube(fields)

	b2 := modbus.NewRequestBuilder(fmt.Sprintf("%s:%d", alfamera.network.GetIp(), alfamera.network.GetPort()), 1)
	b2 = alfamera.timeToDate(b2)
	requests, _ = b2.ReadHoldingRegistersRTU()

	alfamera.logger.Debug("%s", len(requests))
	resp, err = client.Do(context.Background(), requests[0])
	alfamera.logger.Debug("%s", requests[0])
	if err != nil {
		alfamera.logger.Debug("%s", err)

		return &alfamera.data, err
	}
	fields, err = requests[0].ExtractFields(resp.(modbus.RegistersResponse), true)
	if err != nil {
		panic(err)
	}
	alfamera.logger.Debug("0")
	alfamera.logger.Debug("%d", len(fields))

	alfamera.convertDateTime(fields)

	b3 := modbus.NewRequestBuilder(fmt.Sprintf("%s:%d", alfamera.network.GetIp(), alfamera.network.GetPort()), 1)
	b3 = alfamera.serialToString(b3)
	requests, _ = b3.ReadHoldingRegistersRTU()

	resp, err = client.Do(context.Background(), requests[0])
	alfamera.logger.Debug("1")
	if err != nil {
		return &alfamera.data, err
	}
	fields, _ = requests[0].ExtractFields(resp.(modbus.RegistersResponse), true)
	alfamera.logger.Debug("2")
	alfamera.logger.Debug("%s", alfamera.arrToSerial(fields))
	alfamera.arrToSerial(fields)
	return &alfamera.data, nil
}

func (alfamera *Alfamera) timeToDate(b *modbus.Builder) *modbus.Builder {

	timeNow := uint16(0xEF50)

	requests := b.
		Add(b.Byte(timeNow+0, true)).
		Add(b.Byte(timeNow+0, false)).
		Add(b.Byte(timeNow+1, true)).
		Add(b.Byte(timeNow+1, false))

	return requests
}

func (alfamera *Alfamera) arrDateTime(dataTime int, fields []modbus.FieldValue) uint32 {

	baseTime := 0
	timeConvert := baseTime + dataTime*4
	arr := [4]byte{
		fields[timeConvert+2].Value.(byte),
		fields[timeConvert+3].Value.(byte),
		fields[timeConvert+0].Value.(byte),
		fields[timeConvert+1].Value.(byte),
	}

	sum := convert.ToLong(arr)
	return sum
}

func (alfamera *Alfamera) convertDateTime(fields []modbus.FieldValue) {

	timestamp := alfamera.arrDateTime(0, fields)

	i, err := strconv.ParseInt(strconv.FormatUint(uint64(timestamp), 10), 10, 64)
	if err != nil {
		panic(err)
	}

	tm := time.Unix(i, 0)
	alfamera.data.Time = tm

}

func (alfamera *Alfamera) serialToString(b *modbus.Builder) *modbus.Builder {

	serialRegistor := uint16(0xEF04)

	requests := b.
		Add(b.Byte(serialRegistor+0, true)).
		Add(b.Byte(serialRegistor+0, false)).
		Add(b.Byte(serialRegistor+1, true)).
		Add(b.Byte(serialRegistor+1, false))

	return requests
}
func (alfamera *Alfamera) arrToSerial(fields []modbus.FieldValue) [4]byte {
	alfamera.logger.Debug("3")
	alfamera.logger.Debug("%s,%s,%s,%s",
		fields[0].Value,
		fields[1].Value,
		fields[2].Value,
		fields[3].Value,
	)
	return [4]byte{fields[1].Value.(byte), fields[0].Value.(byte), fields[3].Value.(byte), fields[2].Value.(byte)}

}

func (alfamera *Alfamera) arrToProperty(properNumber int, fields []modbus.FieldValue) [4]byte {
	basePropert := 0
	tubePropert := basePropert + properNumber*4

	return [4]byte{fields[tubePropert+2].Value.(byte), fields[tubePropert+3].Value.(byte), fields[tubePropert+0].Value.(byte), fields[tubePropert+1].Value.(byte)}
}

func (alfamera *Alfamera) convertDataTube(fields []modbus.FieldValue) {

	alfamera.data.Systems[0].GM1 = (math.Float32frombits(ToLong(alfamera.arrToProperty(0, fields))) / 1000)
	alfamera.data.Systems[0].Q1 = float64(0.000000238843 * math.Float32frombits(ToLong(alfamera.arrToProperty(1, fields))))
	alfamera.data.Systems[0].P1 = math.Float32frombits(ToLong(alfamera.arrToProperty(2, fields)))
	alfamera.data.Systems[0].T1 = math.Float32frombits(ToLong(alfamera.arrToProperty(3, fields)))

}

func (alfamera *Alfamera) addTube(tubeNumber uint16, b *modbus.Builder) *modbus.Builder {

	baseRegistor := uint16(0x1600)
	tubeRegistor := baseRegistor + tubeNumber*14
	requests := b.
		//qm
		Add(b.Byte(tubeRegistor+0, true)).
		Add(b.Byte(tubeRegistor+0, false)).
		Add(b.Byte(tubeRegistor+1, true)).
		Add(b.Byte(tubeRegistor+1, false)).
		//W
		Add(b.Byte(tubeRegistor+2, true)).
		Add(b.Byte(tubeRegistor+2, false)).
		Add(b.Byte(tubeRegistor+3, true)).
		Add(b.Byte(tubeRegistor+3, false)).
		//p
		Add(b.Byte(tubeRegistor+4, true)).
		Add(b.Byte(tubeRegistor+4, false)).
		Add(b.Byte(tubeRegistor+5, true)).
		Add(b.Byte(tubeRegistor+5, false)).
		//T
		Add(b.Byte(tubeRegistor+6, true)).
		Add(b.Byte(tubeRegistor+6, false)).
		Add(b.Byte(tubeRegistor+7, true)).
		Add(b.Byte(tubeRegistor+7, false))
		//Читатель регистров
		// ReadHoldingRegistersRTU()
	return requests
}

// ToLong Функция перевода
func ToLong(bytes [4]byte) uint32 {
	var amount uint32 = 0
	for i := 0; i <= 3; i++ {
		amount += uint32(uint32(bytes[i]) << uint32(8*(3-i)))
	}
	return amount
}
