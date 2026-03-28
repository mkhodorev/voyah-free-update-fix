FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y --no-install-recommends openssh-server ca-certificates && \
    rm -rf /var/lib/apt/lists/*

RUN mkdir -p /var/run/sshd /mnt/ota/data/fota

RUN echo "root:12345" | chpasswd

RUN sed -i 's/^#\?PermitRootLogin .*/PermitRootLogin yes/' /etc/ssh/sshd_config && \
    sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication yes/' /etc/ssh/sshd_config && \
    sed -i 's/^#\?KbdInteractiveAuthentication .*/KbdInteractiveAuthentication no/' /etc/ssh/sshd_config && \
    sed -i 's/^#\?UsePAM .*/UsePAM no/' /etc/ssh/sshd_config

COPY tests/docker/entrypoint.sh /entrypoint.sh
COPY tests/fixtures/context.db /mnt/ota/data/fota/context.db
RUN chmod +x /entrypoint.sh && chmod 0644 /mnt/ota/data/fota/context.db

EXPOSE 22

ENTRYPOINT ["/entrypoint.sh"]
