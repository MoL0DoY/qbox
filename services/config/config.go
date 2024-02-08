package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"qBox/drivers"
	"qBox/drivers/alfamera"
	"qBox/drivers/skm2"
	"qBox/drivers/skm2m"
	"qBox/drivers/tem104k"
	"qBox/drivers/tem104m"
	"qBox/models"
)

// Карта зарегистрированных драйверов.
// Примечание: Добавляя новые драйвера, необходимо добавить описание в HELP для флага type
var driversMap = [16]models.IDeviceDriver{
	new(skm2.SKM),
	new(drivers.SKU02B),
	new(drivers.Tem104),
	new(drivers.TEM05OLD),
	new(drivers.SKU02),
	new(drivers.TM3),
	new(drivers.TEM104M1),
	new(drivers.Tem104s1),
	new(drivers.TESMART01),
	new(drivers.SKU02K),
	new(drivers.SKU02B7B),
	new(tem104m.TEM104M),
	new(tem104k.Tem104K),
	new(drivers.TEM104M2),
	new(skm2m.SKM),
	new(alfamera.Alfamera),
}

const VersionCoreApp = "0.0.5"

type Config struct {
	log           bool
	dev           bool
	hostPort      string
	deviceType    int
	format        string
	counterNumber uint
	unitQInt      uint
}

func (cS Config) IsOnLog() bool {
	return cS.log
}

func (cS Config) IsDevEnv() bool {
	return cS.dev
}

func (cS Config) GetHostPort() string {
	return cS.hostPort
}

func (cS Config) GetCounterNumber() byte {
	return byte(cS.counterNumber)
}

func (cS *Config) GetDriver() (models.IDeviceDriver, error) {
	for i, driver := range driversMap {
		if i == cS.deviceType {
			return driver, nil
		}
	}
	return nil, errors.New("задан не верный драйвер устройства. Список драйверов доступен по флагу \"-help\" или \"-h\"")
}

func (cS Config) GetFormatter() models.Formatter {
	switch cS.format {
	case "json":
		return new(models.JsonFormat)
	}
	return new(models.TextFormat)
}

// Возвращает конфигурацию для единиц измерения энергии.
// Если неверно заданы, то возвращается ошибка и ГКал.
func (cS Config) GetUnitQ() (models.UnitQEnum, error) {
	switch cS.unitQInt {
	case 1:
		return models.Gcal, nil
	case 2:
		return models.GJ, nil
	case 3:
		return models.KWh, nil
	case 0:
		return models.MWh, nil
	}

	return models.Gcal, errors.New("единицы измерения энергии выставлены не правильно. Список возможных вариантов доступен по флагу \"-help\" или \"-h\"")
}

// Инициализация конфигурации системы. Используются возможности стандартного пакета "flag"
// Ошибки игнорируются для этого метода, т.к. flag.Parse() сам грохает терминал при ошибках.
// Валидация должна производиться в методах Config.
func InitConfig() Config {
	configService := new(Config)

	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stdout, "Утилита qBox предоставляет возможность опрашивать теплосчётчики, используя различные драйверы.")
		_, _ = fmt.Fprintf(os.Stdout, "Использование: %s -type=[драйвер] [другие настройки] ipAddress:port\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stdout, "Например: %s -type=2 192.168.12.1\n", os.Args[0])
		_, _ = fmt.Fprintln(os.Stdout, "")
		_, _ = fmt.Fprintln(os.Stdout, "Список доступных настроек:")
		_, _ = fmt.Fprintln(os.Stdout, "")
		flag.PrintDefaults()
	}

	flag.BoolVar(
		&configService.log,
		"log",
		true,
		"Флаг настройки лога. Флаг принимает значения 1, 0. Если выключено, то в лог попадают только сообщения \n\t"+
			"об ошибках программы. Лог файла создаётся в директории из которой запущена утилита.")

	flag.BoolVar(
		&configService.dev,
		"dev",
		false,
		"Флаг состояния системы. Включенный флаг подразумевает, что программа запущена в режиме разработчика,\n\t"+
			"отладочная информация о работе утилиты будет попадать в лог.\n\t"+
			"Выключенный флаг - режим производства, отладачная информация в логах скрыта.\n\t"+
			"Принимает значения 1, 0.")

	flag.IntVar(
		&configService.deviceType,
		"type",
		9999,
		"Обязательный атрибут. Тип теплосчётчика, в зависимости от выбранного типа используется тот или иной драйвер\n\t"+
			"Доступные типы(драйвера):"+
			"\n\t   0 - СКМ-2"+
			"\n\t   1 - SKU-02-B (5b)."+
			"\n\t   2 - ТЭМ-104"+
			"\n\t   3 - ТЭМ-05"+
			"\n\t   4 - SKU-02"+
			"\n\t   5 - ИСТОК TM3"+
			"\n\t   6 - TEM-104M-1"+
			"\n\t   7 - TEM-104-1"+
			"\n\t   8 - TEM-104-1 ТЭСМАРТ (РФ)"+
			"\n\t   9 - SKU-02-K"+
			"\n\t   10 - SKU-02-B (7b)"+
			"\n\t   11 - TEM-104M"+
			"\n\t   12 - TEM-104k"+
			"\n\t   13 - TEM-104M2"+
			"\n\t   14 - SKM2M."+
			"\n\t   15 - alfa-mera")

	flag.UintVar(
		&configService.counterNumber,
		"number",
		0,
		"Номер теплосчётчика. Может принимать значения от 0 до 255")

	flag.StringVar(
		&configService.format,
		"format",
		"text",
		"Формат вывода результата. По умолчанию текстовый вид \"text\". Также доступен формат \"json\"")

	flag.UintVar( // Значения такие же как models.unitQ
		&configService.unitQInt,
		"unitQ",
		1,
		"Единицы измерения энергии. По умолчанию вывод значения энергии предоставляется в ГКал. Возможно:"+
			"\n\t   1 - ГКал"+
			"\n\t   2 - ГДж"+
			"\n\t   3 - КВт"+
			"\n\t   0 - МВт")

	var versionFlag *bool
	versionFlag = flag.Bool("version", false, "Версия "+VersionCoreApp)

	flag.Parse()

	if *versionFlag {
		fmt.Println(flag.Lookup("version").Usage)
		os.Exit(0)
	}

	configService.hostPort = flag.Arg(0)

	return *configService
}
