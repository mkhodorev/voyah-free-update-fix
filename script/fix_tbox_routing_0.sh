#!/bin/sh

chmod +x ./adb > /dev/null 2>&1
xattr -d com.apple.quarantine ./adb > /dev/null 2>&1
export PATH=$PATH:.

if [[ ! -f "adb" || ! -r "adb" || ! -x "adb" ]]; then
echo .
echo Отсутствует файл adb, вы не разархивировали архив или не перешли в директорию скрипта
echo Разархивируйте архив и запустите этот скрипт ещё раз
echo .
read -p "Теперь это окно можно закрыть" -n1 -s
exit 1
fi

echo Подготовливаем adb к работе
echo .
echo Если тут зависло:
echo .
echo 1 Проверьте включен ли USB Debugging, https://voyahchat.ru/common/usb-debugging
echo Иногда помогает включить/выключить USB Debugging несколько раз
echo .
echo 2 Проверьте подходит ли кабель, https://voyahchat.ru/common/cable
echo На странице описаны все варианты использования кабеля, наиболее рабочий вариант кабель A-A
echo Кабель C-A не работает на Маке, нужно использовать кабель A-A и переходник C-A
echo Сначала вставляется переходник C-A в Мак, потом в него вставляется кабель A-A и вставляется в машину
echo .
echo 3 Перезагрузите компьютер и запустите скрипт заново
echo Проверьте, что у вас не держит ADB соединение что-то другое, например, установка Cunba
echo .
adb wait-for-device
if [ $? -ne 0 ]; then
echo .
echo Не удалось выполнить ADB соединение с машиной
echo Возможно у вас уже запущено какое-то другое ADB соединение
echo .
read -p "Теперь это окно можно закрыть" -n1 -s
exit 1
fi
echo adb к работе подготовлено

echo .
echo Переключение adb в режим root
echo .
adb -d root
sleep 5

echo Ожидание устройства
echo .
adb -d wait-for-device

echo Отключение verity
echo .
adb -d disable-verity
sleep 5

echo Машина сейчас перезагрузится
echo .
adb -d reboot
echo Дождитесь, пока устройство перезагрузится
echo .
read -p "После полной загрузки мультимедиа нажмите Enter для продолжения "

echo .
echo Ожидание устройства
echo .
adb -d wait-for-device
echo Переключение adb в режим root
echo .
adb -d root
sleep 5

echo Ожидание устройства
echo .
adb -d wait-for-device

echo Перемонтирование файловой системы
echo .
adb -d remount
sleep 5

echo Ожидание устройства
echo .
adb -d wait-for-device

echo .
echo Настраиваем iptables
adb -d shell iptables -P INPUT ACCEPT
adb -d shell iptables -P FORWARD ACCEPT
adb -d shell iptables -P OUTPUT ACCEPT
adb -d shell iptables -F
adb -d shell iptables -X
adb -d shell iptables -Z
adb -d shell sysctl -w net.ipv4.ip_forward=1
adb -d shell iptables -t nat -A PREROUTING -i wlan0 -j DNAT --to-destination 172.16.104.20
adb -d shell iptables -A FORWARD -i wlan0 -d 172.16.104.20 -j ACCEPT
adb -d shell iptables -t nat -A POSTROUTING -o wlan0 -j MASQUERADE

echo .
read -p "Теперь это окно можно закрыть" -n1 -s

