package main

import (
	"flag"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
)

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

func createARecord(hostedZoneID, DNSName, hostName, localIP string, logLevel *aws.LogLevelType) error {
	sess, err := session.NewSession(&aws.Config{Credentials: credentials.NewEnvCredentials(), LogLevel: logLevel})
	if err != nil {
		return err
	}
	r53 := route53.New(sess)
	// This API call creates a new DNS record for this host
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(hostName + "." + DNSName),
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

func createCNAMERecord(hostedZoneID, DNSName, hostName, localName string, logLevel *aws.LogLevelType) error {
	sess, err := session.NewSession(&aws.Config{Credentials: credentials.NewEnvCredentials(), LogLevel: logLevel})
	if err != nil {
		return err
	}
	r53 := route53.New(sess)
	// This API call creates a new DNS record for this host
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(hostName + "." + DNSName),
						// It creates an A record with the IP of the host running the agent
						Type: aws.String(route53.RRTypeCname),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(localName),
							},
						},
						SetIdentifier: aws.String(hostName),
						// TTL=0 to avoid DNS caches
						TTL:    aws.Int64(defaultTTL),
						Weight: aws.Int64(defaultWeight),
					},
				},
			},
			Comment: aws.String("Host CName Record Created"),
		},
		HostedZoneId: aws.String(hostedZoneID),
	}
	_, err = r53.ChangeResourceRecordSets(params)
	logErrorNoFatal(err)
	if err == nil {
		log.Print("Record " + hostName + " created, resolves to " + localName)
	}
	return err
}

func main() {
	logLevel := aws.LogLevel(aws.LogOff)
	var err error
	var sum int
	var zoneID string

	var hostname = flag.String("hostname", "", "which name to use for the new entry")
	var cname = flag.Bool("cname", false, "whether to create CNAME record instead of an A record. (will use public hostname instead of IP)")
	var debug = flag.Bool("debug", false, "enable aws logging")
	var DNSName = flag.String("zonename", "", "which zone to use for registering records")
	var zoneIDArg = flag.String("zoneId", "", "route53 zone id which to use for registering records (instead of searching zone by name)")
	flag.Parse()

	if *debug {
		logLevel = aws.LogLevel(aws.LogDebugWithRequestErrors | aws.LogDebugWithHTTPBody)
	}

	if *DNSName == "" && *zoneIDArg == "" {
		log.Fatal("Either zonename or zoneId parameter is required. It sepecifies the zone in which record is added!")
	}

	if *hostname == "" {
		log.Fatal("Either host or ip params are needed!")
	}

	if *zoneIDArg == "" {
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
	} else {
		zoneID = "/hostedzone/" + *zoneIDArg
	}

	sess, err := session.NewSession()
	logErrorAndFail(err)
	metadataClient := ec2metadata.New(sess)

	if *cname == false {
		localIP, err := metadataClient.GetMetadata("/local-ipv4")
		logErrorAndFail(err)
		if err = createARecord(zoneID, *DNSName, *hostname, localIP, logLevel); err != nil {
			log.Print("Error creating host A record")
		}
	} else {
		localName, err := metadataClient.GetMetadata("/public-hostname")
		logErrorAndFail(err)
		if err = createCNAMERecord(zoneID, *DNSName, *hostname, localName, logLevel); err != nil {
			log.Print("Error creating host CName record")
		}

	}

}
