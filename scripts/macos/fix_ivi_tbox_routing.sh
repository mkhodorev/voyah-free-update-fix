#!/bin/bash

chmod +x ./adb > /dev/null 2>&1
xattr -d com.apple.quarantine ./adb > /dev/null 2>&1
export PATH="$PATH:."

TBOX_IP="172.16.104.20"

if [[ ! -f "./adb" || ! -r "./adb" || ! -x "./adb" ]]; then
  echo
  echo "Отсутствует файл adb в текущей папке"
  echo
  read -p "Теперь это окно можно закрыть" -n1 -s
  exit 1
fi

echo
echo "Ожидание ADB-устройства..."
./adb wait-for-device
if [ $? -ne 0 ]; then
  echo
  echo "Не удалось подключиться по ADB"
  echo
  read -p "Теперь это окно можно закрыть" -n1 -s
  exit 1
fi

echo
echo "Переключение adbd в root..."
./adb -d root
sleep 3
./adb -d wait-for-device

echo
echo "Проверка доступности tbox с мультимедиа..."
./adb -d shell ping -c 2 "$TBOX_IP"
if [ $? -ne 0 ]; then
  echo
  echo "Мультимедиа не видит tbox по адресу $TBOX_IP"
  echo "Скорее всего проблема не только в firewall."
  echo
  read -p "Теперь это окно можно закрыть" -n1 -s
  exit 1
fi

echo
echo "Включаем IPv4 forwarding..."
./adb -d shell "sysctl -w net.ipv4.ip_forward=1"

echo
echo "Добавляем только точечные правила для tbox..."
./adb -d shell "iptables -C FORWARD -d $TBOX_IP -j ACCEPT 2>/dev/null || iptables -A FORWARD -d $TBOX_IP -j ACCEPT"
./adb -d shell "iptables -C FORWARD -s $TBOX_IP -j ACCEPT 2>/dev/null || iptables -A FORWARD -s $TBOX_IP -j ACCEPT"

echo
echo "Готово."
echo 
ping "$TBOX_IP"
echo
read -p "Теперь это окно можно закрыть" -n1 -s