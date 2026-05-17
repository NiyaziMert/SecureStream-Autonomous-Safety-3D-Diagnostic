package main

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacemetering"
)

// AWSMarketplaceClient: AWS Marketplace Metering API ile konuşan yardımcı istemci.
type AWSMarketplaceClient struct {
	client *marketplacemetering.Client
}

// NewAWSMarketplaceClient: AWS SDK yapılandırmasını yükler ve istemciyi başlatır.
func NewAWSMarketplaceClient() (*AWSMarketplaceClient, error) {
	// AWS Marketplace genellikle global olarak us-east-1 bölgesinde çalışır.
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %v", err)
	}

	return &AWSMarketplaceClient{
		client: marketplacemetering.NewFromConfig(cfg),
	}, nil
}

// ResolveCustomer: Müşteri Marketplace üzerinden satın alım yaptıktan sonra SaaS sitemize yönlendirildiğinde
// URL'de gelen "x-amzn-marketplace-token" parametresini çözümler. Müşterinin benzersiz AWS ID'sini döner.
func (aws *AWSMarketplaceClient) ResolveCustomer(registrationToken string) (string, string, error) {
	input := &marketplacemetering.ResolveCustomerInput{
		RegistrationToken: &registrationToken,
	}

	result, err := aws.client.ResolveCustomer(context.TODO(), input)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve customer registration token: %v", err)
	}

	// Müşterinin benzersiz CustomerIdentifier'ı ve satın aldığı ProductCode döner.
	return *result.CustomerIdentifier, *result.ProductCode, nil
}

// MeterUsage: Müşterinin yaptığı kullanımı AWS fatura paneline yansıtır.
// - productCode: AWS Marketplace'teki ürün kodunuz.
// - dimension: AWS portalında tanımladığımız fiyatlandırma kalemi (örn: "ActiveNodes" veya "DataIngestedGB")
// - quantity: Kullanım miktarı
func (aws *AWSMarketplaceClient) MeterUsage(productCode string, dimension string, quantity int32) error {
	now := time.Now().UTC()
	dryRun := false
	input := &marketplacemetering.MeterUsageInput{
		ProductCode:    &productCode,
		UsageDimension: &dimension,
		Timestamp:      &now,
		UsageQuantity:  &quantity,
		DryRun:         &dryRun,
	}

	_, err := aws.client.MeterUsage(context.TODO(), input)
	if err != nil {
		return fmt.Errorf("failed to meter usage on AWS Billing: %v", err)
	}

	fmt.Printf("📊 [AWS Billing] Reported %d units of '%s' for product %s\n", quantity, dimension, productCode)
	return nil
}

// StartAWSUsageReporting: Arka planda saatlik olarak müşterinin aktif node sayısını AWS'ye bildiren döngü.
func StartAWSUsageReporting(aws *AWSMarketplaceClient, productCode string) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		fmt.Printf("⏰ [AWS Billing] Starting background usage reporter loop for product: %s\n", productCode)
		for range ticker.C {
			// Simülatörde çalışan aktif node sayısı (örn: 39)
			// Gerçek projede bu veri veritabanından dinamik çekilir.
			var activeNodes int32 = 39 

			err := aws.MeterUsage(productCode, "ActiveNodes", activeNodes)
			if err != nil {
				fmt.Printf("⚠️  [AWS Billing Error] Failed to report background usage: %v\n", err)
			}
		}
	}()
}
