package main

import (
	"flag"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

const workerTimeout = 180 * time.Second
const defaultTTL = 0
const defaultWeight = 1

func logErrorAndFail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func logErrorNoFatal(err error) {
	if err != nil {
		log.Print(err)
	}
}

func getDNSHostedZoneID(DNSName string) (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	r53 := route53.New(sess)
	params := &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(DNSName),
	}

	zones, err := r53.ListHostedZonesByName(params)

	if err == nil {
		if len(zones.HostedZones) > 0 {
			return aws.StringValue(zones.HostedZones[0].Id), nil
		}
	}

	return "", err
}

func createARecord(hostedZoneID, DNSName, hostName, localIP string) error {
	sess, err := session.NewSession()
	if err != nil {
		return err
	}
	r53 := route53.New(sess)
	// This API call creates a new DNS record for this host
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(route53.ChangeActionCreate),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(strings.Split(hostName, ".")[0] + "." + DNSName),
						// It creates an A record with the IP of the host running the agent
						Type: aws.String(route53.RRTypeA),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(localIP),
							},
						},
						SetIdentifier: aws.String(hostName),
						// TTL=0 to avoid DNS caches
						TTL:    aws.Int64(defaultTTL),
						Weight: aws.Int64(defaultWeight),
					},
				},
			},
			Comment: aws.String("Host A Record Created"),
		},
		HostedZoneId: aws.String(hostedZoneID),
	}
	_, err = r53.ChangeResourceRecordSets(params)
	logErrorNoFatal(err)
	if err == nil {
		log.Print("Record " + hostName + " created, resolves to " + localIP)
	}
	return err
}

func main() {
	var err error
	var sum int
	var zoneID string

	var hostname = flag.String("hostname", "", "to use for registering the A record")
	var DNSName = flag.String("zonename", "", "to use for registering the A record")
	flag.Parse()

	if *DNSName == "" || *hostname == "" {
		log.Fatal("Both hostname and zonename params are needed!")
	}

	for {
		// We try to get the Hosted Zone Id using exponential backoff
		zoneID, err = getDNSHostedZoneID(*DNSName)
		if err == nil {
			break
		}
		if sum > 8 {
			logErrorAndFail(err)
		}
		time.Sleep(time.Duration(sum) * time.Second)
		sum += 2
	}

	sess, err := session.NewSession()
	logErrorAndFail(err)
	metadataClient := ec2metadata.New(sess)

	localIP, err := metadataClient.GetMetadata("/local-ipv4")
	logErrorAndFail(err)

	if err = createARecord(zoneID, *DNSName, *hostname, localIP); err != nil {
		log.Print("Error creating host A record")
	}
}