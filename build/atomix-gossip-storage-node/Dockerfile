FROM alpine:3.9

USER nobody

ADD build/_output/atomix-gossip-storage-node /usr/local/bin/atomix-gossip-storage-node

ENTRYPOINT ["atomix-gossip-storage-node"]
