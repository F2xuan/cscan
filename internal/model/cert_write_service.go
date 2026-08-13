package model

import (
	"context"
	"strconv"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"go.mongodb.org/mongo-driver/mongo"
)

// ScannerCert 扫描器证书数据传输对象（避免循环依赖）
type ScannerCert struct {
	Host         string
	Port         int
	Authority    string
	Subject      CertNameInfo
	SubjectDN    string
	Issuer       CertNameInfo
	IssuerDN     string
	SerialNumber string
	SigAlg       string
	NotBefore    time.Time
	NotAfter     time.Time
	Version      int
	SANs         []string
	Fingerprints map[string]string
	IsSelfSigned bool
}

// CertWriteService 证书写入服务，封装完整的证书保存业务逻辑
type CertWriteService struct {
	db        *mongo.Database
	certModel *CertModel
}

// NewCertWriteService 创建证书写入服务
func NewCertWriteService(db *mongo.Database) *CertWriteService {
	return &CertWriteService{
		db:        db,
		certModel: NewCertModel(db),
	}
}

// SaveCerts 保存证书列表（完整业务逻辑从 API handler 层迁移）
func (s *CertWriteService) SaveCerts(ctx context.Context, mainTaskID string, certs []*ScannerCert) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if len(certs) == 0 {
		return nil
	}

	docs := make([]*Cert, 0, len(certs))
	now := time.Now()

	for _, c := range certs {
		authority := c.Authority
		if authority == "" {
			authority = c.Host + ":" + strconv.Itoa(c.Port)
		}

		docs = append(docs, &Cert{
			TaskId:       mainTaskID,
			Host:         c.Host,
			Port:         c.Port,
			Authority:    authority,
			Subject:      c.Subject,
			SubjectDN:    c.SubjectDN,
			Issuer:       c.Issuer,
			IssuerDN:     c.IssuerDN,
			SerialNumber: c.SerialNumber,
			SigAlg:       c.SigAlg,
			NotBefore:    c.NotBefore,
			NotAfter:     c.NotAfter,
			Version:      c.Version,
			SANs:         c.SANs,
			Fingerprints: c.Fingerprints,
			IsSelfSigned: c.IsSelfSigned,
			CreateTime:   now,
			UpdateTime:   now,
		})
	}

	if err := s.certModel.EnsureIndexes(ctx); err != nil {
		logx.Errorf("[CertWriteService] EnsureIndexes failed: %v", err)
		return err
	}

	if err := s.certModel.UpsertMany(ctx, docs); err != nil {
		logx.Errorf("[CertWriteService] UpsertMany failed: %v", err)
		return err
	}

	logx.Infof("[CertWriteService] SaveCerts: saved %d certificates for task=%s", len(certs), mainTaskID)
	return nil
}
