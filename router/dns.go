package router

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

func addDNSRecord(db *sql.DB, domain string) {
	var internalip string
	var externalip string
	isinternal, err := IsInternalRouter(db, domain)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if isinternal {
		internalip = os.Getenv("PRIVATE_SNI_VIP_INTERNAL")
	} else {
		internalip = os.Getenv("PRIVATE_SNI_VIP")
		externalip = os.Getenv("PUBLIC_SNI_VIP")
	}

	svc := route53.New(session.New(&aws.Config{
		Region: aws.String("us-west-2"),
	}))
	params := &route53.ListHostedZonesByNameInput{
		DNSName:  aws.String(os.Getenv("DOMAIN_NAME")),
		MaxItems: aws.String("2"),
	}
	resp, err := svc.ListHostedZonesByName(params)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, element := range resp.HostedZones {
		if *element.Config.PrivateZone {
			fmt.Println(*element.Id)
			paramschange := addRecordSet(*element.Id, domain, internalip)
			respchange, err := svc.ChangeResourceRecordSets(paramschange)

			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println(respchange)

		}
		if !*element.Config.PrivateZone && !isinternal {
			fmt.Println(*element.Id)
			paramschange := addRecordSet(*element.Id, domain, externalip)
			respchange, err := svc.ChangeResourceRecordSets(paramschange)

			if err != nil {
				fmt.Println(err)
				return
			}

			fmt.Println(respchange)

		}
	}

}

func addRecordSet(id string, dnsname string, ip string) *route53.ChangeResourceRecordSetsInput {
	paramschange := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String("UPSERT"),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(dnsname),
						Type: aws.String("A"),
						TTL:  aws.Int64(300),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(ip),
							},
						},
					},
				},
			},
		},
		HostedZoneId: aws.String(id),
	}
	return paramschange
}
