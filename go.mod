module github.com/atomix/atomix-gossip-storage

go 1.12

require (
	github.com/atomix/atomix-api/go v0.4.5
	github.com/atomix/atomix-controller v0.5.0
	github.com/atomix/atomix-go-framework v0.6.12
	github.com/gogo/protobuf v1.3.1
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/mod v0.1.1-0.20191107180719-034126e5016b // indirect
	golang.org/x/tools v0.0.0-20200207183749-b753a1ba74fa // indirect
	google.golang.org/grpc v1.33.2
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/utils v0.0.0-20191114184206-e782cd3c129f
	sigs.k8s.io/controller-runtime v0.5.2
)

replace github.com/atomix/atomix-go-framework => ../atomix-go-node
