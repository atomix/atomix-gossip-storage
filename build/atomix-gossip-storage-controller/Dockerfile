FROM alpine:3.9

USER nobody

ADD build/_output/atomix-gossip-storage-controller /usr/local/bin/atomix-gossip-storage-controller

ENTRYPOINT ["atomix-gossip-storage-controller"]
