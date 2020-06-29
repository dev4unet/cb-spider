package api

import (
	"errors"
	"io"
	"time"

	gc "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/common"
	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/config"
	"github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/logger"
	pb "github.com/cloud-barista/cb-spider/api-runtime/grpc-runtime/stub/cbspider"
	"github.com/cloud-barista/cb-spider/interface/api/request"

	"google.golang.org/grpc"
)

// ===== [ Constants and Variables ] =====

// ===== [ Types ] =====

// CIMApi - CIM API 구조 정의
type CIMApi struct {
	gConf        *config.GrpcConfig
	conn         *grpc.ClientConn
	jaegerCloser io.Closer
	clientCIM    pb.CIMClient
	requestCIM   *request.CIMRequest
	inType       string
	outType      string
}

// ===== [ Implementations ] =====

// SetServerAddr - Spider 서버 주소 설정
func (cim *CIMApi) SetServerAddr(addr string) error {
	if addr == "" {
		return errors.New("parameter is empty")
	}

	cim.gConf.GSL.SpiderSrv.Addr = addr
	return nil
}

// GetServerAddr - Spider 서버 주소 값 조회
func (cim *CIMApi) GetServerAddr() (string, error) {
	return cim.gConf.GSL.SpiderSrv.Addr, nil
}

// SetTLSCA - TLS CA 설정
func (cim *CIMApi) SetTLSCA(tlsCAFile string) error {
	if tlsCAFile == "" {
		return errors.New("parameter is empty")
	}

	if cim.gConf.GSL.SpiderCli.TLS == nil {
		cim.gConf.GSL.SpiderCli.TLS = &config.TLSConfig{}
	}

	cim.gConf.GSL.SpiderCli.TLS.TLSCA = tlsCAFile
	return nil
}

// GetTLSCA - TLS CA 값 조회
func (cim *CIMApi) GetTLSCA() (string, error) {
	if cim.gConf.GSL.SpiderCli.TLS == nil {
		return "", nil
	}

	return cim.gConf.GSL.SpiderCli.TLS.TLSCA, nil
}

// SetTimeout - Timeout 설정
func (cim *CIMApi) SetTimeout(timeout time.Duration) error {
	cim.gConf.GSL.SpiderCli.Timeout = timeout
	return nil
}

// GetTimeout - Timeout 값 조회
func (cim *CIMApi) GetTimeout() (time.Duration, error) {
	return cim.gConf.GSL.SpiderCli.Timeout, nil
}

// SetJWTToken - JWT 인증 토큰 설정
func (cim *CIMApi) SetJWTToken(token string) error {
	if token == "" {
		return errors.New("parameter is empty")
	}

	if cim.gConf.GSL.SpiderCli.Interceptors == nil {
		cim.gConf.GSL.SpiderCli.Interceptors = &config.InterceptorsConfig{}
		cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}
	if cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT == nil {
		cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT = &config.AuthJWTConfig{}
	}

	cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT.JWTToken = token
	return nil
}

// GetJWTToken - JWT 인증 토큰 값 조회
func (cim *CIMApi) GetJWTToken() (string, error) {
	if cim.gConf.GSL.SpiderCli.Interceptors == nil {
		return "", nil
	}
	if cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT == nil {
		return "", nil
	}

	return cim.gConf.GSL.SpiderCli.Interceptors.AuthJWT.JWTToken, nil
}

// SetConfigPath - 환경설정 파일 설정
func (cim *CIMApi) SetConfigPath(configFile string) error {
	logger := logger.NewLogger()

	// Viper 를 사용하는 설정 파서 생성
	parser := config.MakeParser()

	var (
		gConf config.GrpcConfig
		err   error
	)

	if configFile == "" {
		logger.Error("Please, provide the path to your configuration file")
		return errors.New("configuration file are not specified")
	}

	logger.Debug("Parsing configuration file: ", configFile)
	if gConf, err = parser.GrpcParse(configFile); err != nil {
		logger.Error("ERROR - Parsing the configuration file.\n", err.Error())
		return err
	}

	// SPIDER SERVER 필수 입력 항목 체크
	spidersrv := gConf.GSL.SpiderSrv

	if spidersrv == nil {
		return errors.New("spidersrv field are not specified")
	}

	if spidersrv.Addr == "" {
		return errors.New("spidersrv.addr field are not specified")
	}

	// SPIDER CLIENT 필수 입력 항목 체크
	spidercli := gConf.GSL.SpiderCli

	if spidercli == nil {
		gConf.GSL.SpiderCli = &config.GrpcClientConfig{Timeout: 90 * time.Second}
		spidercli = gConf.GSL.SpiderCli
	}

	if spidercli.TLS != nil {
		if spidercli.TLS.TLSCA == "" {
			return errors.New("spidercli.tls.tls_ca field are not specified")
		}
	}

	if spidercli.Interceptors != nil {
		if spidercli.Interceptors.AuthJWT != nil {
			if spidercli.Interceptors.AuthJWT.JWTToken == "" {
				return errors.New("spidercli.interceptors.auth_jwt.jwt_token field are not specified")
			}
		}
		if spidercli.Interceptors.Opentracing != nil {
			if spidercli.Interceptors.Opentracing.Jaeger != nil {
				if spidercli.Interceptors.Opentracing.Jaeger.Endpoint == "" {
					return errors.New("spidercli.interceptors.opentracing.jaeger.endpoint field are not specified")
				}
			}
		}
	}

	cim.gConf = &gConf
	return nil
}

// Open - 연결 설정
func (cim *CIMApi) Open() error {

	spidersrv := cim.gConf.GSL.SpiderSrv
	spidercli := cim.gConf.GSL.SpiderCli

	// grpc 커넥션 생성
	cbconn, closer, err := gc.NewCBConnection(spidersrv.Addr, spidercli)
	if err != nil {
		return err
	}

	if closer != nil {
		cim.jaegerCloser = closer
	}

	cim.conn = cbconn.Conn

	// grpc 클라이언트 생성
	cim.clientCIM = pb.NewCIMClient(cim.conn)

	// grpc 호출 Wrapper
	cim.requestCIM = &request.CIMRequest{Client: cim.clientCIM, Timeout: spidercli.Timeout, InType: cim.inType, OutType: cim.outType}

	return nil
}

// Close - 연결 종료
func (cim *CIMApi) Close() {
	if cim.conn != nil {
		cim.conn.Close()
	}
	if cim.jaegerCloser != nil {
		cim.jaegerCloser.Close()
	}

	cim.jaegerCloser = nil
	cim.conn = nil
	cim.clientCIM = nil
	cim.requestCIM = nil
}

// SetInType - 입력 문서 타입 설정 (json/yaml)
func (cim *CIMApi) SetInType(in string) error {
	if in == "json" {
		cim.inType = in
	} else if in == "yaml" {
		cim.inType = in
	} else {
		return errors.New("input type is not supported")
	}

	if cim.requestCIM != nil {
		cim.requestCIM.InType = cim.inType
	}

	return nil
}

// GetInType - 입력 문서 타입 값 조회
func (cim *CIMApi) GetInType() (string, error) {
	return cim.inType, nil
}

// SetOutType - 출력 문서 타입 설정 (json/yaml)
func (cim *CIMApi) SetOutType(out string) error {
	if out == "json" {
		cim.outType = out
	} else if out == "yaml" {
		cim.outType = out
	} else {
		return errors.New("output type is not supported")
	}

	if cim.requestCIM != nil {
		cim.requestCIM.OutType = cim.outType
	}

	return nil
}

// GetOutType - 출력 문서 타입 값 조회
func (cim *CIMApi) GetOutType() (string, error) {
	return cim.outType, nil
}

// ListCloudOS -Cloud OS 목록
func (cim *CIMApi) ListCloudOS() (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	return cim.requestCIM.ListCloudOS()
}

// CreateCloudDriver - Cloud Driver 생성
func (cim *CIMApi) CreateCloudDriver(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.CreateCloudDriver()
}

// ListCloudDriver -Cloud Driver 목록
func (cim *CIMApi) ListCloudDriver() (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	return cim.requestCIM.ListCloudDriver()
}

// GetCloudDriver - Cloud Driver 조회
func (cim *CIMApi) GetCloudDriver(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.GetCloudDriver()
}

// GetCloudDriverByParam - Cloud Driver 조회
func (cim *CIMApi) GetCloudDriverByParam(driverName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"DriverName":"` + driverName + `"}`
	result, err := cim.requestCIM.GetCloudDriver()
	cim.SetInType(holdType)

	return result, err
}

// DeleteCloudDriver - Cloud Driver 삭제
func (cim *CIMApi) DeleteCloudDriver(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.DeleteCloudDriver()
}

// DeleteCloudDriverByParam - Cloud Driver 삭제
func (cim *CIMApi) DeleteCloudDriverByParam(driverName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"DriverName":"` + driverName + `"}`
	result, err := cim.requestCIM.DeleteCloudDriver()
	cim.SetInType(holdType)

	return result, err
}

// CreateCredential - Credential 생성
func (cim *CIMApi) CreateCredential(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.CreateCredential()
}

// ListCredential -Credential 목록
func (cim *CIMApi) ListCredential() (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	return cim.requestCIM.ListCredential()
}

// GetCredential - Credential 조회
func (cim *CIMApi) GetCredential(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.GetCredential()
}

// GetCredentialByParam - Credential 조회
func (cim *CIMApi) GetCredentialByParam(credentialName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"CredentialName":"` + credentialName + `"}`
	result, err := cim.requestCIM.GetCredential()
	cim.SetInType(holdType)

	return result, err
}

// DeleteCredential - Credential 삭제
func (cim *CIMApi) DeleteCredential(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.DeleteCredential()
}

// DeleteCredentialByParam - Credential 삭제
func (cim *CIMApi) DeleteCredentialByParam(credentialName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"CredentialName":"` + credentialName + `"}`
	result, err := cim.requestCIM.DeleteCredential()
	cim.SetInType(holdType)

	return result, err
}

// CreateRegion - Region 생성
func (cim *CIMApi) CreateRegion(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.CreateRegion()
}

// ListRegion - Region 목록
func (cim *CIMApi) ListRegion() (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	return cim.requestCIM.ListRegion()
}

// GetRegion - Region 조회
func (cim *CIMApi) GetRegion(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.GetRegion()
}

// GetRegionByParam - Region 조회
func (cim *CIMApi) GetRegionByParam(regionName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"RegionName":"` + regionName + `"}`
	result, err := cim.requestCIM.GetRegion()
	cim.SetInType(holdType)

	return result, err
}

// DeleteRegion - Region 삭제
func (cim *CIMApi) DeleteRegion(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.DeleteRegion()
}

// DeleteRegionByParam - Region 삭제
func (cim *CIMApi) DeleteRegionByParam(regionName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"RegionName":"` + regionName + `"}`
	result, err := cim.requestCIM.DeleteRegion()
	cim.SetInType(holdType)

	return result, err
}

// CreateConnectionConfig - Connection Config 생성
func (cim *CIMApi) CreateConnectionConfig(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.CreateConnectionConfig()
}

// ListConnectionConfig - Connection Config 목록
func (cim *CIMApi) ListConnectionConfig() (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	return cim.requestCIM.ListConnectionConfig()
}

// GetConnectionConfig - Connection Config 조회
func (cim *CIMApi) GetConnectionConfig(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.GetConnectionConfig()
}

// GetConnectionConfigByParam - Connection Config 조회
func (cim *CIMApi) GetConnectionConfigByParam(configName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"ConfigName":"` + configName + `"}`
	result, err := cim.requestCIM.GetConnectionConfig()
	cim.SetInType(holdType)

	return result, err
}

// DeleteConnectionConfig - Connection Config 삭제
func (cim *CIMApi) DeleteConnectionConfig(doc string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	cim.requestCIM.InData = doc
	return cim.requestCIM.DeleteConnectionConfig()
}

// DeleteConnectionConfigByParam - Connection Config 삭제
func (cim *CIMApi) DeleteConnectionConfigByParam(configName string) (string, error) {
	if cim.requestCIM == nil {
		return "", errors.New("The Open() function must be called")
	}

	holdType, _ := cim.GetInType()
	cim.SetInType("json")
	cim.requestCIM.InData = `{"ConfigName":"` + configName + `"}`
	result, err := cim.requestCIM.DeleteConnectionConfig()
	cim.SetInType(holdType)

	return result, err
}

// ===== [ Private Functions ] =====

// ===== [ Public Functions ] =====

// NewCloudInfoManager - CIM API 객체 생성
func NewCloudInfoManager() (cim *CIMApi) {

	cim = &CIMApi{}
	cim.gConf = &config.GrpcConfig{}
	cim.gConf.GSL.SpiderSrv = &config.GrpcServerConfig{}
	cim.gConf.GSL.SpiderCli = &config.GrpcClientConfig{}

	cim.jaegerCloser = nil
	cim.conn = nil
	cim.clientCIM = nil
	cim.requestCIM = nil

	cim.inType = "json"
	cim.outType = "json"

	return
}
