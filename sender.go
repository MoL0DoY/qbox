package main
//
//import (
//	"fmt"
//	"time"
//)
//import (
//	"net"
//)
//
//func main() {
//	ip := net.ParseIP("10.12.184.164")
//	addr := &net.TCPAddr{
//		IP:   ip,
//		Port: 37772,
//	}
//
//	conn, err := net.DialTCP("tcp", nil, addr)
//	if err != nil {
//		fmt.Printf("%s\n", err.Error())
//		return
//	}
//
//	defer func() {
//		err = conn.Close()
//		if err != nil {
//			fmt.Printf("%s\n", err.Error())
//		} else {
//			fmt.Printf("Соединение закрыто\n")
//		}
//	}()
//
//	//command := []byte{0x55, 0x01, 0xFE, 0x0F, 0x01, 0x03, 0x00, 0x00, 0x04}
//	//command := []byte{0x55, 0x01, 0xFE, 0x00, 0x00, 0x00}
//	command := []byte{0x10, 0x40, 0x02, 0x42, 0x16}
//	request := append(command)
//	fmt.Printf("Отправляем %X\n", request)
//	_, err = conn.Write(request)
//
//	if err != nil {
//		fmt.Printf("%s\n", err.Error())
//		return
//	}
//
//	/**
//	Минимальная скорость 2400 бит (300 байт) в секунду.
//	Максимальная - 9600 бит (1200 байт) в секунду.
//	Пинг 1.6 секунд на МТС. 0.3 секунды у Велкома.
//	*/
//	buffer := make([]byte, 1200)
//	err = conn.SetDeadline(time.Now().Add(3 * time.Second))
//	if err != nil {
//		fmt.Printf("%s\n", err.Error())
//		return
//	}
//
//	for {
//		fmt.Printf("Читаю\n")
//		n, err := conn.Read(buffer)
//		if err != nil {
//			fmt.Printf("%s\n", err.Error())
//			break
//		}
//		fmt.Printf("Получено %d байт:\n%X\n", n, buffer[:n])
//	}
//
//	fmt.Printf("закончено\n")
//}
