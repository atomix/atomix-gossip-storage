FROM alpine:3.9

USER nobody

ADD build/_output/atomix-gossip-storage-driver /usr/local/bin/atomix-gossip-storage-driver

ENTRYPOINT ["atomix-gossip-storage-driver"]
