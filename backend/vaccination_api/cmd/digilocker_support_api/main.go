package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"github.com/divoc/api/config"
	"github.com/divoc/kernel_library/services"
	"github.com/gorilla/mux"
	"github.com/signintech/gopdf"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const CertificateEntity = "VaccinationCertificate"

type Certificate struct {
	Context           []string `json:"@context"`
	Type              []string `json:"type"`
	CredentialSubject struct {
		Type        string `json:"type"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		Gender      string `json:"gender"`
		Age         int    `json:"age"`
		Nationality string `json:"nationality"`
	} `json:"credentialSubject"`
	Issuer       string    `json:"issuer"`
	IssuanceDate string `json:"issuanceDate"`
	Evidence     []struct {
		ID             string    `json:"id"`
		FeedbackURL    string    `json:"feedbackUrl"`
		InfoURL        string    `json:"infoUrl"`
		Type           []string  `json:"type"`
		Batch          string    `json:"batch"`
		Vaccine        string    `json:"vaccine"`
		Manufacturer   string    `json:"manufacturer"`
		Date           time.Time `json:"date"`
		EffectiveStart string    `json:"effectiveStart"`
		EffectiveUntil string    `json:"effectiveUntil"`
		Verifier       struct {
			Name string `json:"name"`
		} `json:"verifier"`
		Facility struct {
			Name    string `json:"name"`
			Address struct {
				StreetAddress  string `json:"streetAddress"`
				StreetAddress2 string `json:"streetAddress2"`
				District       string `json:"district"`
				City           string `json:"city"`
				AddressRegion  string `json:"addressRegion"`
				AddressCountry string `json:"addressCountry"`
			} `json:"address"`
		} `json:"facility"`
	} `json:"evidence"`
	NonTransferable string `json:"nonTransferable"`
	Proof           struct {
		Type               string    `json:"type"`
		Created            time.Time `json:"created"`
		VerificationMethod string    `json:"verificationMethod"`
		ProofPurpose       string    `json:"proofPurpose"`
		Jws                string    `json:"jws"`
	} `json:"proof"`
}

type PullURIRequest struct {
	XMLName    xml.Name `xml:"PullURIRequest"`
	Text       string   `xml:",chardata"`
	Ns2        string   `xml:"ns2,attr"`
	Ver        string   `xml:"ver,attr"`
	Ts         string   `xml:"ts,attr"`
	Txn        string   `xml:"txn,attr"`
	OrgId      string   `xml:"orgId,attr"`
	Format     string   `xml:"format,attr"`
	DocDetails struct {
		Text         string `xml:",chardata"`
		DocType      string `xml:"DocType"`
		DigiLockerId string `xml:"DigiLockerId"`
		UID          string `xml:"UID"`
		FullName     string `xml:"FullName"`
		DOB          string `xml:"DOB"`
		Photo        string `xml:"Photo"`
		UDF1         string `xml:"UDF1"`
		UDF2         string `xml:"UDF2"`
		UDF3         string `xml:"UDF3"`
		UDFn         string `xml:"UDFn"`
	} `xml:"DocDetails"`
}

type PullURIResponse struct {
	XMLName        xml.Name `xml:"PullURIResponse"`
	Text           string   `xml:",chardata"`
	Ns2            string   `xml:"ns2,attr"`
	ResponseStatus struct {
		Text   string `xml:",chardata"`
		Status string `xml:"Status,attr"`
		Ts     string `xml:"ts,attr"`
		Txn    string `xml:"txn,attr"`
	} `xml:"ResponseStatus"`
	DocDetails struct {
		Text         string `xml:",chardata"`
		DocType      string `xml:"DocType"`
		DigiLockerId string `xml:"DigiLockerId"`
		UID          string `xml:"UID"`
		FullName     string `xml:"FullName"`
		DOB          string `xml:"DOB"`
		UDF1         string `xml:"UDF1"`
		UDF2         string `xml:"UDF2"`
		URI          string `xml:"URI"`
		DocContent   string `xml:"DocContent"`
		DataContent  string `xml:"DataContent"`
	} `xml:"DocDetails"`
}

func ValidMAC(message, messageMAC, key []byte) bool {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	expectedMAC := mac.Sum(nil)
	if log.IsLevelEnabled(log.InfoLevel) {
		log.Infof("Expected mac %s but got %s", base64.StdEncoding.EncodeToString(expectedMAC), base64.StdEncoding.EncodeToString(messageMAC))
	}
	return hmac.Equal(messageMAC, expectedMAC)
}
func uriRequest(w http.ResponseWriter, req *http.Request) {
	log.Info("Got request ")
	requestBuffer := make([]byte, 2048)
	n, _ := req.Body.Read(requestBuffer)
	log.Infof("Read %d bytes ", n)
	request := string(requestBuffer)
	log.Infof("Request body %s", request)

	hmacDigest := req.Header.Get(config.Config.Digilocker.AuthKeyName)
	hmacSignByteArray, e := base64.StdEncoding.DecodeString(hmacDigest)
	if e != nil {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("Error in verifying request signature"))
		return
	}

	if ValidMAC(requestBuffer, hmacSignByteArray, []byte(config.Config.Digilocker.AuthHMACKey)) {

		xmlRequest := PullURIRequest{}
		if err := xml.Unmarshal(requestBuffer, &xmlRequest); err != nil {
			log.Errorf("Error in marshalling request from the digilocker %+v", err)
		} else {

			response := PullURIResponse{}
			response.ResponseStatus.Ts = xmlRequest.Ts
			response.ResponseStatus.Txn = xmlRequest.Txn
			response.ResponseStatus.Status = "1"
			response.DocDetails.DocType = config.Config.Digilocker.DocType
			response.DocDetails.DigiLockerId = xmlRequest.DocDetails.DigiLockerId
			response.DocDetails.FullName = xmlRequest.DocDetails.FullName
			response.DocDetails.DOB = xmlRequest.DocDetails.DOB

			certBundle := getCertificate(xmlRequest.DocDetails.FullName, xmlRequest.DocDetails.DOB,
				xmlRequest.DocDetails.UID, xmlRequest.DocDetails.UDF1)
			if certBundle != nil {
				response.DocDetails.URI = certBundle.Uri
				if xmlRequest.Format == "pdf" || xmlRequest.Format == "both" {
					if pdfBytes, err := getPdfCertificate(certBundle.signedJson); err != nil {
						log.Errorf("Error in creating certificate pdf")
					} else {
						pdfContent := pdfBytes // todo get pdf
						response.DocDetails.DocContent = base64.StdEncoding.EncodeToString(pdfContent)
					}
				}
				if xmlRequest.Format == "both" || xmlRequest.Format == "xml" {
					certificateId := certBundle.certificateId
					xmlCert := "<certificate id=\"" + certificateId + "\"><![CDATA[" + certBundle.signedJson + "]]></certificate>"
					response.DocDetails.DataContent = base64.StdEncoding.EncodeToString([]byte(xmlCert))
				}

				if responseBytes, err := xml.Marshal(response); err != nil {
					log.Errorf("Error while serializing xml")
				} else {
					w.WriteHeader(200)
					_, _ = w.Write(responseBytes)
					return
				}
			}
			w.WriteHeader(500)
		}
	} else {
		w.WriteHeader(401)
		_, _ = w.Write([]byte("Unauthorized"))
	}

}

type VaccinationCertificateBundle struct {
	certificateId string
	Uri           string
	signedJson    string
	pdf           []byte
}

func fetchCertificateFromRegistry(filter map[string]interface{}) (map[string]interface{}, error) {
	return services.QueryRegistry(CertificateEntity, filter)
}

func getCertificate(fullName string, dob string, aadhaar string, phoneNumber string) *VaccinationCertificateBundle {
	filter := map[string]interface{}{
		"name": map[string]interface{}{
			"eq": fullName,
		},
		"mobile": map[string]interface{}{
			"eq": phoneNumber,
		},
	}
	certificateFromRegistry, err := fetchCertificateFromRegistry(filter)
	if err == nil {
		certificateArr := certificateFromRegistry[CertificateEntity].([]interface{})
		if len(certificateArr) > 0 {
			certificateObj := certificateArr[0].(map[string]interface{})
			log.Infof("certificate resp %v", certificateObj)
			var cert VaccinationCertificateBundle
			cert.certificateId = certificateObj["certificateId"].(string)
			cert.Uri = "https://moh.india.gov/vc/233423"
			cert.signedJson = certificateObj["certificate"].(string)
			return &cert
		}
	}
	return nil
}

func getCertificateAsPdf(certificateText string) ([]byte, error) {
	var certificate Certificate
	if err := json.Unmarshal([]byte(certificateText), &certificate); err != nil {
		log.Error(err)
		return nil, err
	}

	pdf := gopdf.GoPdf{}
	//pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: 595.28, H: 841.89}}) //595.28, 841.89 = A4
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	if err := pdf.AddTTFFont("wts11", "./Roboto-Medium.ttf"); err != nil {
		log.Print(err.Error())
		return nil, err
	}
	/* if err := pdf.AddTTFFont("arapey", "./Arapey-Italic.ttf"); err != nil {
		log.Print(err.Error())
		return nil, err
	} */
	tpl1 := pdf.ImportPage("Certificate provisional_Final_11 Jan(4).pdf", 1, "/MediaBox")
	// Draw pdf onto page
	pdf.UseImportedTemplate(tpl1, 0, 0, 580, 0)

	if err := pdf.SetFont("wts11", "", 10); err != nil {
		log.Print(err.Error())
		return nil, err
	}

	/*if err := pdf.SetFont("arapey", "", 14); err != nil {
		log.Print(err.Error())
		return nil, err
	}*/
	offsetX := 280.0
	offsetY := 361.0
	displayLabels := []string{certificate.CredentialSubject.Name,
		strconv.Itoa(certificate.CredentialSubject.Age) + " Years",
		certificate.CredentialSubject.Gender,
		"234234298374293",
		formatId(certificate.CredentialSubject.ID),
		"",
		"", //blank line
		certificate.Evidence[0].Vaccine,
		fomratDate(certificate.Evidence[0].Date) + " (Batch no. " + certificate.Evidence[0].Batch + ")",
		"To be taken 28 days after 1st Dose",
		"",
		formatFacilityAddress(certificate),
		certificate.Evidence[0].Verifier.Name,
	}
	//offsetYs := []float64{0, 20.0, 40.0, 60.0}
	for i := 0; i < len(displayLabels); i++ {
		pdf.SetX(offsetX)
		pdf.SetY(offsetY + float64(i)*22.0)
		pdf.Cell(nil, displayLabels[i])
	}

	e := pasteQrCodeOnPage(certificateText, &pdf)
	if e != nil {
		return nil, e
	}

	//pdf.Image("qr.png", 200, 50, nil)
	//pdf.WritePdf("certificate.pdf")
	var b bytes.Buffer
	pdf.Write(&b)
	return b.Bytes(), nil
}

func formatFacilityAddress(certificate Certificate) string {
	return certificate.Evidence[0].Facility.Name + ", " + certificate.Evidence[0].Facility.Address.District + ", " + certificate.Evidence[0].Facility.Address.AddressRegion
}

func fomratDate(date time.Time) string {
	return date.Format("02 Jan 2006")
}

func formatId(identity string) string {
	split := strings.Split(identity, ":")
	lastFragment := split[len(split)-1]
	if strings.Contains(identity, "aadhaar") {
		return "XXXX XXXX XXXX " + lastFragment[len(lastFragment)-4:]
	}
	return lastFragment
}

func pasteQrCodeOnPage(certificateText string, pdf *gopdf.GoPdf) error {
	qrCode, err := qrcode.New(certificateText, qrcode.Medium)
	if err != nil {
		return err
	}
	imageBytes, err := qrCode.PNG(-2)
	holder, err := gopdf.ImageHolderByBytes(imageBytes)
	err = pdf.ImageByHolder(holder, 400, 30, nil)
	if err != nil {
		log.Errorf("Error while creating QR code")
	}
	return nil
}

func getPDFHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	preEnrollmentCode := vars["preEnrollmentCode"]
	filter := map[string]interface{}{
		"preEnrollmentCode": map[string]interface{}{
			"eq": preEnrollmentCode,
		},
	}
	certificateFromRegistry, err := fetchCertificateFromRegistry(filter)
	if err == nil {
		certificateArr := certificateFromRegistry[CertificateEntity].([]interface{})
		if len(certificateArr) > 0 {
			certificateObj := certificateArr[0].(map[string]interface{})
			log.Infof("certificate resp %v", certificateObj)
			signedJson := certificateObj["certificate"].(string)
			if pdfBytes, err := getPdfCertificate(signedJson); err != nil {
				log.Errorf("Error in creating certificate pdf")
			} else {
				//w.Header().Set("Content-Disposition", "attachment; filename=certificate.pdf")
				//w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
				//w.Header().Set("Content-Length", string(len(pdfBytes)))
				w.WriteHeader(200)
				_, _ = w.Write(pdfBytes)
				return
			}
		}
	}
}

func main() {
	config.Initialize()
	log.Info("Running digilocker support api")
	r := mux.NewRouter()
	r.HandleFunc("/pullUriRequest", uriRequest).Methods("POST")
	r.HandleFunc("/certificatePDF/{preEnrollmentCode}", getPDFHandler).Methods("GET")
	http.Handle("/", r)
	_ = http.ListenAndServe(":8003", nil)
}
