package main

import (
	"os/signal"
	logPackage "qBox/services/log"
	"syscall"
)
import netService "qBox/services/net"
import configPackage "qBox/services/config"

import (
	"os"
	"qBox/models"
)

/*
  ██████╗ ██████╗  ██████╗ ██╗  ██╗
 ██╔═══██╗██╔══██╗██╔═══██╗╚██╗██╔╝
 ██║   ██║██████╔╝██║   ██║ ╚███╔╝
 ██║▄▄ ██║██╔══██╗██║   ██║ ██╔██╗
 ╚██████╔╝██████╔╝╚██████╔╝██╔╝ ██╗
  ╚══▀▀═╝ ╚═════╝  ╚═════╝ ╚═╝  ╚═╝
 предоставляет возможность опрашивать теплосчётчики, используя различные драйверы.
 @author Evgeniy Tkachenko, et.coder@gmail.com

Компиляция https://go.dev/doc/tutorial/compile-install

компилировать для WIN32
go env -w GOARCH=386&&go env -w GOOS=windows&&go build

компилировать для WIN64
go env -w GOARCH=amd64&&go env -w GOOS=windows&&go build

компилировать для Linux32
go env -w GOARCH=386&&go env -w GOOS=linux&&go build

компилировать для Linux64
go env -w GOARCH=amd64&&go env -w GOOS=linux&&go build
*/
func main() {
	var err error

	// ИНИЦИАЛИЗАЦИЯ КОМПОНЕНТОВ
	configService := configPackage.InitConfig()

	logger := logPackage.LoggerService{}
	err = logger.Open(configService.IsDevEnv(), !configService.IsOnLog())
	if err != nil {
		panic(err)
	}

	driver, err := configService.GetDriver()
	if err != nil {
		logger.Check("driver")
		logger.Fatal(err.Error())
		logger.Close()
		return
	}

	host, port, err := netService.SplitHostPort(configService.GetHostPort())
	if err != nil {
		logger.Fatal(err.Error())
		logger.Close()
		return
	}

	network := *netService.NewNetwork(host, port, logger)

	// ОБРАБОТКА ЗАВЕРШЕНИЯ ПРОГРАММЫ
	defer func() {
		if network.IsConnected() {
			err = network.Close()
			if err != nil {
				logger.Fatal(err.Error())
				return
			}
		}
		logger.Close()
	}()

	signalChanel := make(chan os.Signal, 1)
	signal.Notify(signalChanel, syscall.SIGINT, syscall.SIGTERM)
	go terminate(signalChanel, &network, &logger)

	// РАБОТА С ДРАЙВЕРОМ
	logger.Check("driver")
	logger.Info("Инициализация драйвера")
	err = driver.Init(configService.GetCounterNumber(), &network, &logger) // TODO: Добавить таймаут, через conn::SetDeadline
	if err != nil {
		logger.Fatal(err.Error())
		return
	}

	logger.Info("Чтение текущих данных") // TODO: Добавить таймаут
	deviceData, err := driver.Read()
	if err != nil {
		logger.Fatal(err.Error())
		formatter := configService.GetFormatter()
		formatter.Render(os.Stdout, deviceData)
		return
	}

	// TODO: Можно закрыть соединение.
	logger.Check("app")
	logger.Info("Подготовка к выводу данных")

	logger.Info("Приведение значения энергии к нужным единицам измерения")
	switch deviceData.UnitQ {
	case models.MWh:
		logger.Info("Единицы измерения энергии по протоколу МВт")
	case models.KWh:
		logger.Info("Единицы измерения энергии по протоколу КВт")
	case models.GJ:
		logger.Info("Единицы измерения энергии по протоколу ГДж")
	case models.Gcal:
		logger.Info("Единицы измерения энергии по протоколу ГКал")
	}
	unitQ, err := configService.GetUnitQ()
	if err != nil {
		logger.Notice(err.Error())
	}
	deviceData.ChangeUnitQ(unitQ)

	logger.Info("Получение формата результата")
	formatter := configService.GetFormatter()

	logger.Info("Вывод данных")
	formatter.Render(os.Stdout, deviceData)
}

// Функция будет вызываться, когда срабатывают ОС сигналы SIGINT или SIGTERM
// См. https://en.wikipedia.org/wiki/Signal_(IPC)
func terminate(signalChanel chan os.Signal, network *netService.Network, logger *logPackage.LoggerService) {
	for {
		sig := <-signalChanel
		logger.Check("app")
		logger.Notice("OS сигнал: " + sig.String())
		logger.Close()
		if network.IsConnected() {
			err := network.Close()
			if err != nil {

			}
		}
		os.Exit(0)
	}
}
