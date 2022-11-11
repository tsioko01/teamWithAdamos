package geo

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/hailocab/go-geoindex"
	"github.com/opentracing/opentracing-go"
	pb "github.com/ucy-coast/hotel-app/internal/geo/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Geo implements the geo service
type Geo struct {
	port      int
	addr      string
	dbsession *DatabaseSession
	tracer    opentracing.Tracer
	geoidx    *geoindex.ClusteringIndex
}

const (
	maxSearchRadius  = 10
	maxSearchResults = 5
)

// NewGeo returns a new server
func NewGeo(a string, p int, db *DatabaseSession, tr opentracing.Tracer) *Geo {
	return &Geo{
		addr:      a,
		port:      p,
		dbsession: db,
		tracer:    tr,
		geoidx:    db.newGeoIndex(),
	}
}

// Run starts the server
func (s *Geo) Run() error {
	if s.port == 0 {
		return fmt.Errorf("server port must be set")
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Timeout: 120 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			PermitWithoutStream: true,
		}),
		grpc.UnaryInterceptor(
			otgrpc.OpenTracingServerInterceptor(s.tracer),
		),
	}

	// Create an instance of the gRPC server
	srv := grpc.NewServer(opts...)

	// Register our service implementation with the gRPC server
	pb.RegisterGeoServer(srv, s) /*?????????*/

	// Register reflection service on gRPC server.
	reflection.Register(srv)

	// Listen for client requests
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Accept and serve incoming client requests 
	log.Printf("Start Geo server. Addr: %s:%d\n", s.addr, s.port)
	return srv.Serve(lis)
}

// Nearby returns all hotels within a given distance.
func (s *Geo) Nearby(ctx context.Context, req *pb.Request) (*pb.Result, error) {
	var (
		points = s.getNearbyPoints(float64(req.Lat), float64(req.Lon))
		res    = &pb.Result{}
	)
	for _, p := range points {
		res.HotelIds = append(res.HotelIds, p.Id())
	}

	return res, nil
}

func (s *Geo) getNearbyPoints(lat, lon float64) []geoindex.Point {
	center := &geoindex.GeoPoint{
		Pid:  "",
		Plat: lat,
		Plon: lon,
	}

	return s.geoidx.KNearest(
		center,
		maxSearchResults,
		geoindex.Km(maxSearchRadius), func(p geoindex.Point) bool {
			return true
		},
	)
}

// Implement Point interface
func (p *point) Lat() float64 { return p.Plat }
func (p *point) Lon() float64 { return p.Plon }
func (p *point) Id() string   { return p.Pid }
