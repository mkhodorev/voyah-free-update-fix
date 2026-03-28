#!/bin/bash

chmod +x ./adb > /dev/null 2>&1
xattr -d com.apple.quarantine ./adb > /dev/null 2>&1
export PATH="$PATH:."

TBOX_IP="172.16.104.20"
WIFI_IF="wlan0"

if [[ ! -f "./adb" || ! -r "./adb" || ! -x "./adb" ]]; then
  echo
  echo "Отсутствует файл adb в текущей папке"
  echo
  read -p "Теперь это окно можно закрыть" -n1 -s
  exit 1
fi

echo
echo "Ожидание ADB-устройства..."
./adb wait-for-device || exit 1

echo
echo "Переключение adbd в root..."
./adb -d root
sleep 3
./adb -d wait-for-device

echo
echo "Проверка, что мультимедиа видит tbox..."
./adb -d shell ping -c 2 "$TBOX_IP"
if [ $? -ne 0 ]; then
  echo
  echo "Мультимедиа не видит tbox по адресу $TBOX_IP"
  echo
  read -p "Теперь это окно можно закрыть" -n1 -s
  exit 1
fi

echo
echo "Включаем IPv4 forwarding..."
./adb -d shell "sysctl -w net.ipv4.ip_forward=1"

echo
echo "Добавляем точечные правила NAT/forward только для tbox..."
./adb -d shell "iptables -C FORWARD -d $TBOX_IP -j ACCEPT 2>/dev/null || iptables -A FORWARD -d $TBOX_IP -j ACCEPT"
./adb -d shell "iptables -C FORWARD -s $TBOX_IP -j ACCEPT 2>/dev/null || iptables -A FORWARD -s $TBOX_IP -j ACCEPT"
./adb -d shell "iptables -t nat -C POSTROUTING -d $TBOX_IP -o $WIFI_IF -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -d $TBOX_IP -o $WIFI_IF -j MASQUERADE"

echo
echo "Готово."
echo "Проверьте доступ с ноутбука на http://$TBOX_IP"
echo
read -p "Теперь это окно можно закрыть" -n1 -s