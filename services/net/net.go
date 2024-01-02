package net

import (
	"bytes"
	"errors"
	"io"
	"net"
	"qBox/services/log"
	"strconv"
	"strings"
	"time"
)

const connected = 0x01
const disconnected = 0x02

/**
Сервис для работы с TCP соединением
*/
type Network struct {
	host             string
	port             int
	connection       *net.TCPConn
	logger           log.LoggerService
	connectionStatus byte
}

func NewNetwork(ip string, port int, logger log.LoggerService) *Network {
	return &Network{host: ip, port: port, logger: logger, connectionStatus: disconnected}
}

func (network *Network) IsConnected() bool {
	return network.connectionStatus == connected
}

func (network *Network) Connect() error {
	var err error

	network.logger.Check("netService")

	if network.IsConnected() {
		err = errors.New("установка соединения невозможна, т.к. соединение было установлено ранее")
		network.logger.Fatal(err.Error())
		return err
	}

	ip := net.ParseIP(network.host)
	addr := &net.TCPAddr{
		IP:   ip,
		Port: network.port,
	}

	network.logger.Info("Установка соединения...")
	network.logger.Info("Host: %v Port: %d", network.host, network.port)
	network.connection, err = net.DialTCP("tcp", nil, addr)
	if err == nil {
		network.connectionStatus = connected
		network.logger.Info("Соединение установлено.")
	} else {
		network.logger.Fatal(err.Error())
	}
	return err
}

func (network *Network) Close() error {
	network.logger.Check("netService")

	network.logger.Info("Соединение закрывается.")
	err := network.connection.Close()
	if err == nil {
		network.connectionStatus = disconnected
		network.logger.Info("Соединение закрыто.")
	} else {
		network.logger.Fatal(err.Error())
	}
	return err
}

func (network *Network) Reconnect() {

	network.logger.Check("netService")

	err := network.Close()
	if err != nil {
		return
	}
	network.connectionStatus = disconnected
	err = network.Connect()
	if err != nil {
		return
	}
	network.connectionStatus = connected
}

func (network *Network) RunIO(request Request) ([]byte, error) {

	var err error
	var response []byte

	network.logger.Check("netService")

	if !network.IsConnected() {
		err = network.Connect()
		if err != nil {
			return nil, err
		}
	}

	write := func() error {
		err = network.connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			network.logger.Debug("%s", err.Error())
			return err
		}
		network.logger.Info("Отправка данных...")
		_, err = network.connection.Write(request.Bytes)
		network.logger.Debug("Отправка %d байт: %X", len(request.Bytes), request.Bytes)
		if err != nil {
			network.logger.Debug("%s", err.Error())
		}
		return err
	}

	err = write()
	if err != nil {
		return nil, err
	}

	network.logger.Info("Запускается процесс чтения данных.")
	errorsCount := 0
	for {

		tempResponse, err := network.doRead(request.SecondsReadTimeout)

		if err == io.EOF && request.Reconnect {
			network.logger.Debug("Получен EOF")
			network.Reconnect()
			err = write()
			if err != nil {
				return response, err
			}
			response = []byte{}

		} else if err != nil {

			// При чтении обнаружены ошибки. Определённые ошибки фиксируются, увеличивая счётчик ошибок.
			// Каждый раз при обнаруженной ошибки запускаем write/read заново, если счётчик ошибок не достиг
			// предельного значения

			if strings.Contains(err.Error(), "i/o timeout") {
				if len(response) > 0 {
					// Данные уже пришли, дальше ждать нет причин. Иногда встречаются счётчики
					// с кратковременной памятью, как у Дори :), поэтому дальнейший таймаут вызывает проблемы
					// с дальнейшим обменом данными.
					break // прерываем обмен данными
				} else {
					// данные не приходили, сработал таймаут
					// В данном случае увеличиваем счётчик ошибок, а далее пробуем послать запрос и получить ответ.
					errorsCount++
				}
			} else if strings.Contains(err.Error(), "An existing connection was forcibly closed by the remote host") {
				// RTU сбрасывает соединения на чтение. Точные причины этого состояния не найдены.
				// В данном случае увеличиваем счётчик ошибок, а далее пробуем послать запрос и получить ответ.
				errorsCount++
			}

			network.logger.Debug("%s", err.Error())

			if errorsCount > 3 {
				// Счётчик ошибок достиг предельного значения. Прерываем обмен данными
				break
			}

			if len(response) == 0 {
				// Зафиксирована ошибка, требуется послать запрос заново.
				err = write()
				if err != nil {
					return response, err
				}
			}
		} else {

			if len(request.Bytes) <= len(tempResponse) && bytes.Equal(request.Bytes, tempResponse[:len(request.Bytes)]) {
				network.logger.Debug("В полученных данных обнаружено эхо. Эхо убрано %X", tempResponse[len(request.Bytes):])
				tempResponse = tempResponse[len(request.Bytes):]
			}

			response = append(response, tempResponse...)
			tempResponse = nil

			if request.ControlFunction(response) {
				// контрольная функция выполняется успешно, поэтому нет причин для дальнейшего чтения данных.
				break
			} else {
				network.logger.Debug("Проверка ответа завершилась неудачей. Дочитываем данные.")
			}
		}
	}

	if len(request.Bytes) <= len(response) && bytes.Equal(request.Bytes, response[:len(request.Bytes)]) {
		response = response[len(request.Bytes):]
		network.logger.Debug("Обнаружено ECHO. Данные очищены от ECHO %X", response)
	}

	if !request.ControlFunction(response) && request.Attempts != 0 {
		network.logger.Debug("Проверка ответа завершилась неудачей. Производится повторная попытка.")
		return network.RunIO(Request{
			request.Bytes,
			request.ControlFunction,
			request.Attempts - 1,
			request.Reconnect,
			request.SecondsReadTimeout})
	}

	if !request.ControlFunction(response) {
		network.logger.Debug("Проверка ответа завершилась неудачей. Повторные попытки все исчерпаны.")
		return response, errors.New("получен некорректный ответ")
	}

	network.logger.Debug("Результат - %X", response)
	return response, err
}

func (network *Network) doRead(secondsTimeout uint8) ([]byte, error) {
	var err error
	err = network.setReadTimeout(secondsTimeout)
	if err != nil {
		network.logger.Debug("%s", err.Error())
		return nil, err
	}

	network.logger.Info("чтение данных...")
	/**
	Минимальная скорость 2400 бит (300 байт) в секунду.
	Максимальная - 9600 бит (1200 байт) в секунду.
	Пинг 1.6 секунд на МТС. 0.3 секунды у Велкома.
	В связи с этим подобран буфер и таймаут для выполнения чтения данных
	*/
	buffer := make([]byte, 1200)
	n, err := network.connection.Read(buffer)
	if err != nil {
		return nil, err
	}

	network.logger.Debug("Получено %d байт: %X", n, buffer[:n])
	return buffer[:n], nil
}

func (network *Network) setReadTimeout(seconds uint8) error {
	return network.connection.SetReadDeadline(time.Now().Add(time.Duration(seconds) * time.Second))
}

func SplitHostPort(endpoint string) (host string, port int, err error) {
	host, portString, err := net.SplitHostPort(endpoint)
	if err != nil {
		return
	}
	port, err = strconv.Atoi(portString)
	return
}
