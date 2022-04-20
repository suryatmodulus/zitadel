package saml

import (
	"github.com/caos/zitadel/internal/api/saml/xml/md"
	"github.com/caos/zitadel/internal/api/saml/xml/xml_dsig"
	dsig "github.com/russellhaering/goxmldsig"
	"net/http"
	"net/url"
	"reflect"
	"testing"
)

func TestSSO_getAcsUrlAndBindingForResponse(t *testing.T) {
	type res struct {
		acs     string
		binding string
	}
	type args struct {
		sp             *ServiceProvider
		requestBinding string
	}
	tests := []struct {
		name string
		args args
		res  res
	}{{
		"sp with post and redirect, default used",
		args{
			&ServiceProvider{
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AssertionConsumerService: []md.IndexedEndpointType{
							{Index: "1", IsDefault: "true", Binding: RedirectBinding, Location: "redirect"},
							{Index: "2", Binding: PostBinding, Location: "post"},
						},
					},
				},
			},
			RedirectBinding,
		},
		res{
			acs:     "redirect",
			binding: RedirectBinding,
		},
	},
		{
			"sp with post and redirect, first index used",
			args{
				&ServiceProvider{
					metadata: &md.EntityDescriptorType{
						SPSSODescriptor: &md.SPSSODescriptorType{
							AssertionConsumerService: []md.IndexedEndpointType{
								{Index: "1", Binding: RedirectBinding, Location: "redirect"},
								{Index: "2", Binding: PostBinding, Location: "post"},
							},
						},
					},
				},
				RedirectBinding,
			},
			res{
				acs:     "redirect",
				binding: RedirectBinding,
			},
		},
		{
			"sp with post and redirect, redirect used",
			args{
				&ServiceProvider{
					metadata: &md.EntityDescriptorType{
						SPSSODescriptor: &md.SPSSODescriptorType{
							AssertionConsumerService: []md.IndexedEndpointType{
								{Binding: RedirectBinding, Location: "redirect"},
								{Binding: PostBinding, Location: "post"},
							},
						},
					},
				},
				RedirectBinding,
			},
			res{
				acs:     "redirect",
				binding: RedirectBinding,
			},
		},
		{
			"sp with post and redirect, post used",
			args{
				&ServiceProvider{
					metadata: &md.EntityDescriptorType{
						SPSSODescriptor: &md.SPSSODescriptorType{
							AssertionConsumerService: []md.IndexedEndpointType{
								{Binding: RedirectBinding, Location: "redirect"},
								{Binding: PostBinding, Location: "post"},
							},
						},
					},
				},
				PostBinding,
			},
			res{
				acs:     "post",
				binding: PostBinding,
			},
		},
		{
			"sp with redirect, post used",
			args{
				&ServiceProvider{
					metadata: &md.EntityDescriptorType{
						SPSSODescriptor: &md.SPSSODescriptorType{
							AssertionConsumerService: []md.IndexedEndpointType{
								{Binding: RedirectBinding, Location: "redirect"},
							},
						},
					},
				},
				PostBinding,
			},
			res{
				acs:     "redirect",
				binding: RedirectBinding,
			},
		},
		{
			"sp with post, redirect used",
			args{
				&ServiceProvider{
					metadata: &md.EntityDescriptorType{
						SPSSODescriptor: &md.SPSSODescriptorType{
							AssertionConsumerService: []md.IndexedEndpointType{
								{Binding: PostBinding, Location: "post"},
							},
						},
					},
				},
				RedirectBinding,
			},
			res{
				acs:     "post",
				binding: PostBinding,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acs, binding := getAcsUrlAndBindingForResponse(tt.args.sp, tt.args.requestBinding)
			if acs != tt.res.acs && binding != tt.res.binding {
				t.Errorf("getAcsUrlAndBindingForResponse() got = %v/%v, want %v/%v", acs, binding, tt.res.acs, tt.res.binding)
				return
			}
		})
	}
}

func TestSSO_getAuthRequestFromRequest(t *testing.T) {
	type res struct {
		want *AuthRequestForm
		err  bool
	}
	tests := []struct {
		name string
		arg  *http.Request
		res  res
	}{
		{
			"parsing form error",
			&http.Request{URL: &url.URL{RawQuery: "invalid=%%param"}},
			res{
				nil,
				true,
			},
		},
		{
			"signed redirect binding",
			&http.Request{URL: &url.URL{RawQuery: "SAMLRequest=request&SAMLEncoding=encoding&RelayState=state&SigAlg=alg&Signature=sig"}},
			res{
				&AuthRequestForm{
					AuthRequest: "request",
					Encoding:    "encoding",
					RelayState:  "state",
					SigAlg:      "alg",
					Sig:         "sig",
					Binding:     RedirectBinding,
				},
				false,
			},
		},
		{
			"unsigned redirect binding",
			&http.Request{URL: &url.URL{RawQuery: "SAMLRequest=request&SAMLEncoding=encoding&RelayState=state"}},
			res{
				&AuthRequestForm{
					AuthRequest: "request",
					Encoding:    "encoding",
					RelayState:  "state",
					SigAlg:      "",
					Sig:         "",
					Binding:     RedirectBinding,
				},
				false,
			},
		},
		{
			"post binding",
			&http.Request{
				Form: map[string][]string{
					"SAMLRequest": {"request"},
					"RelayState":  {"state"},
				},
				URL: &url.URL{RawQuery: ""}},
			res{
				&AuthRequestForm{
					AuthRequest: "request",
					Encoding:    "",
					RelayState:  "state",
					SigAlg:      "",
					Sig:         "",
					Binding:     PostBinding,
				},
				false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getAuthRequestFromRequest(tt.arg)
			if (err != nil) != tt.res.err {
				t.Errorf("getAuthRequestFromRequest() error = %v, wantErr %v", err, tt.res.err)
			}
			if !reflect.DeepEqual(got, tt.res.want) {
				t.Errorf("getAuthRequestFromRequest() got = %v, want %v", got, tt.res.want)
			}
		})
	}
}

func TestSSO_certificateCheckNecessary(t *testing.T) {
	type args struct {
		sig      *xml_dsig.SignatureType
		metadata *md.EntityDescriptorType
	}
	tests := []struct {
		name string
		args args
		res  bool
	}{
		{
			"sig nil",
			args{
				sig:      nil,
				metadata: &md.EntityDescriptorType{},
			},
			false,
		},
		{
			"keyinfo nil",
			args{
				sig:      &xml_dsig.SignatureType{KeyInfo: nil},
				metadata: &md.EntityDescriptorType{},
			},
			false,
		},
		{
			"keydescriptor nil",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: nil,
					},
				},
			},
			false,
		},
		{
			"keydescriptor length == 0",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{},
					},
				},
			},
			false,
		},
		{
			"check necessary",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{{Use: "test"}},
					},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authRequestF := func() *xml_dsig.SignatureType {
				return tt.args.sig
			}
			metadataF := func() *md.EntityDescriptorType {
				return tt.args.metadata
			}

			gotF := certificateCheckNecessary(authRequestF, metadataF)
			got := gotF()
			if got != tt.res {
				t.Errorf("certificateCheckNecessary() got = %v, want %v", got, tt.res)
			}
		})
	}
}

func TestSSO_checkCertificate(t *testing.T) {
	type args struct {
		sig      *xml_dsig.SignatureType
		metadata *md.EntityDescriptorType
	}
	tests := []struct {
		name string
		args args
		err  bool
	}{
		{
			"keydescriptor length == 0",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{},
					},
				},
			},
			true,
		},
		{
			"x509data length == 0",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{X509Data: []xml_dsig.X509DataType{}}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{{Use: "test"}},
					},
				},
			},
			true,
		},
		{
			"certificates equal",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{X509Data: []xml_dsig.X509DataType{{X509Certificate: "test"}}}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{{Use: "test", KeyInfo: xml_dsig.KeyInfoType{X509Data: []xml_dsig.X509DataType{{X509Certificate: "test"}}}}},
					},
				},
			},
			false,
		},
		{
			"certificates not equal",
			args{
				sig: &xml_dsig.SignatureType{KeyInfo: &xml_dsig.KeyInfoType{X509Data: []xml_dsig.X509DataType{{X509Certificate: "test1"}}}},
				metadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						KeyDescriptor: []md.KeyDescriptorType{{Use: "test", KeyInfo: xml_dsig.KeyInfoType{X509Data: []xml_dsig.X509DataType{{X509Certificate: "test2"}}}}},
					},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authRequestF := func() *xml_dsig.SignatureType {
				return tt.args.sig
			}
			metadataF := func() *md.EntityDescriptorType {
				return tt.args.metadata
			}

			gotF := checkCertificate(authRequestF, metadataF)
			got := gotF()
			if (got != nil) != tt.err {
				t.Errorf("checkCertificate() got = %v, want %v", got, tt.err)
			}
		})
	}
}

func TestSSO_signatureRedirectVerificationNecessary(t *testing.T) {
	type args struct {
		sig         string
		binding     string
		idpMetadata *md.IDPSSODescriptorType
		spMetadata  *md.EntityDescriptorType
	}
	tests := []struct {
		name string
		args args
		res  bool
	}{
		{
			"redirect signature",
			args{
				sig:     "test",
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			true,
		},
		{
			"redirect AuthnRequestsSigned",
			args{
				sig:     "",
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "true",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			true,
		},
		{
			"redirect WantAuthnRequestsSigned",
			args{
				sig:     "",
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "true",
				},
			},
			true,
		},
		{
			"redirect no signature",
			args{
				sig:     "",
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			false,
		},
		{
			"post all required",
			args{
				sig:     "test",
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "true",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "true",
				},
			},
			false,
		},
		{
			"post nothing required",
			args{
				sig:     "",
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idpMetadataF := func() *md.IDPSSODescriptorType {
				return tt.args.idpMetadata
			}
			spMetadataF := func() *md.EntityDescriptorType {
				return tt.args.spMetadata
			}
			signatureF := func() string {
				return tt.args.sig
			}
			bindingF := func() string {
				return tt.args.binding
			}

			gotF := signatureRedirectVerificationNecessary(idpMetadataF, spMetadataF, signatureF, bindingF)
			got := gotF()
			if got != tt.res {
				t.Errorf("signatureRedirectVerificationNecessary() got = %v, want %v", got, tt.res)
			}
		})
	}
}

func TestSSO_verifyRedirectSignature(t *testing.T) {
	type args struct {
		request    string
		relayState string
		sig        string
		sigAlg     string
		spMetadata string
	}
	tests := []struct {
		name string
		args args
		err  bool
	}{
		{
			"redirect signed request 1",
			args{
				request:    "nJJBj9MwEIX/ijX3No613WStTaSyFaLSwlabwoHb1JlQS45dPBNg/z1qu0iLhHLgas/nN+/53TOO4WTXkxzjM32fiEX9GkNke75oYMrRJmTPNuJIbMXZbv3x0ZqltshMWXyK8AY5zTOnnCS5FEBtNw34fjHcOHS35a0ZzHDo8XAwrrxb1VgPujZlP1RV3ddY3oH6Qpl9ig2YpQa1ZZ5oG1kwSgNGG7PQNwuj97qypbarellX1VdQG2LxEeVCHkVOtihCchiOicWutNamOO9ddN0TqPUfSw8p8jRS7ij/8I4+Pz/+g6611lcYHYPavXp752Pv47f5IA7XIbYf9vvdYvfU7aG9fIa9OMvqfcojyvwj55NzhpdRS1G8vEA7s+dIgj0K3hdvpNrXEnzCkbabXQrevfyHvGSM7CkKqHUI6edDJhRqQPJEULRXyb+r1v4OAAD//w==",
				relayState: "Hv9rftq0AHE47MealTo9m7TCIGhLVedUjmlwyCXLgUepny_c_WOO6f3e",
				sig:        "UE1buXT5lJvUMX5N1baY8OOvoOdsYplqiOdB8VYLUD3CfBt6EHlDta560bnKIovl5/xBsL8hZrMBwZXnzmZ5bNt9RYnSQZNxYXl5t/CnNScbdW4pC8I4gWzTxWmsKCQRBw9JvvpZCKojND1kKT0NMTlOPZHTB+Je8zbR2rNCkY4JePnmOIunOCXvfMpRgMScyFTe/udrLaBQPvVIZ7uE8noGzzANqHAOgS7HqvlLT4jBPd7RO3U/+Vp8mIUH+wkff9iZ/Kp9pambgQ18QJJNTb4By16JtHMqrziSAZX05YXXPyWhdontccZL/kOMHXY1VTaR8vABm/pOaX3GozZEPw==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"redirect signed request 2",
			args{
				request:    "nJJBb9swDIX/isB7YkZt7FmoDWQNhgXo1qDOdtiNttlFgC1lIr2t/35I0gEZMOTQq8RPj+/p3QmNw8GtJt2HJ/4xsaj5PQ5B3PGigikFF0m8uEAji9PONatPD87O0ZEIJ/UxwAVyuM4cUtTYxQHMZl2B72fPXCDa8qZd3ra4RGoXZZE/9/liUWLe5/0iL7qibAnMV07iY6jAzhHMRmTiTRCloBVYtHaGtzOLOyyczd1NPs+X5Tcwaxb1gfRE7lUPLsuG2NGwj6JuiYg2O+6dNc0jmNVfS/cxyDRyajj99B1/eXr4D/0OEc8wdQJm++rtvQ+9D9+vB9Geh8R93O22s+1js4P69Bnu5CyZDzGNpNcfOZ4cMzyNOg7q9QXqK3uOrNST0l12IVW/luAzjbxZb+Pgu5c3yGuiIJ6DglkNQ/x1n5iUK9A0MWT1WfLfqtV/AgAA//8=",
				relayState: "YIz2twuwoPbPXS7oCd9ErSU9qsW2BvPC-STqeCN3EnJHoaUdG__bXIyD",
				sig:        "gnKrz9/UuY9te90EKQiiuOdFvuqszkDeFTDCPww21g301j39VKhMmCNdvnG6inW2W/I2lSFmu147QsIkIqZV55mYKAaQYuuSzcW9Ni0YZeshTNmBf72EUy3ykp58nzQScInTq2iRAUdwSDuL42ScSwOLh/UOvFH9cv6ERIBX9pljh89UbuLrL6cXbAlJofkiKorzGcTZfsATbWsSnAU0G9eBaGoSV2JMgRoLEpYq4J/wPN8fqB8htJ8fla+9BGrnBNGq3T92KvoEjANriMm+s50lko0ENa9KIbNPEh+45zEh/4t1MVIo1cZm82+Im2CT/rPp2s930DHvs4F2vOD8+A==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"redirect signed request 3",
			args{
				request:    "nJJBj9MwEIX/ijX3NJbbpqm1iVS2QlRa2GpTOHCbOgO15NjFMwH236O2i7RIKIe92vP5ved5d4xDONvNKKf4RD9GYlG/hxDZXi4aGHO0CdmzjTgQW3G223x8sGamLTJTFp8ivELO08w5J0kuBVC7bQO+L+a1WRiqjTviqidz7N0a5z2tK1wtK1dXPerFaj6vQH2hzD7FBsxMg9oxj7SLLBilAaONKfSiMPqgV9bUdrGerZfVV1BbYvER5UqeRM62LENyGE6JxS611qa8+C677hHU5m+k+xR5HCh3lH96R5+fHv5D11rrG4yOQe1fsr3zsffx+/RHHG9DbD8cDvti/9gdoL0uw16TZfU+5QFl+pHLie+Lb9dRS1G8PEM74XMgwR4F78pXUu1LCT7hQLvtPgXvnt8gLxkje4oCahNC+nWfCYUakDwSlO1N8t+qtX8CAAD//w==",
				relayState: "iQURykBYIotpOOVTADzkn7WPmpT9DK3tujPKwYKbcVTj84Y4HXSIm2C2",
				sig:        "Kg1KLmUqSMVliymLBwUq09inVVHNx1UON86C3rmAyKXKj6q0av5qwlZova0htjpGqGcyZTEY4gJSM6FLN+bUjP4DVQul96jUr7+AFw4lMma2RrdzEINtzy8KXEHYbMxTTcDr0Mvnn3D7nmUi9inJNJmh4zJJafmQkhok4/DF0c7+AKizQCRIV35JCWf69XxhZFjMzijoKqWrkOSh9id14KktxSaHUyvVRH4LskzPsIuYeysL9xlrS77r3P8zuaU0EbaESwbTp/q/q7hEq6yH6vg1TXcJCIFPZOqTo0/00UAGie/ExmBp/OebvlHjgJP7g/bF6vK5kGFnxQi4To0Y1A==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"redirect false signed request",
			args{
				request:    "nJJBj9MwEIX/ijX3NJbbpqm1iVS2QlRa2GpTOHCbOgO15NjFMwH236O2i7RIKIe92vP5ved5d4xDONvNKKf4RD9GYlG/hxDZXi4aGHO0CdmzjTgQW3G223x8sGamLTJTFp8ivELO08w5J0kuBVC7bQO+L+a1WRiqjTviqidz7N0a5z2tK1wtK1dXPerFaj6vQH2hzD7FBsxMg9oxj7SLLBilAaONKfSiMPqgV9bUdrGerZfVV1BbYvER5UqeRM62LENyGE6JxS611qa8+C677hHU5m+k+xR5HCh3lH96R5+fHv5D11rrG4yOQe1fsr3zsffx+/RHHG9DbD8cDvti/9gdoL0uw16TZfU+5QFl+pHLie+Lb9dRS1G8PEM74XMgwR4F78pXUu1LCT7hQLvtPgXvnt8gLxkje4oCahNC+nWfCYUakDwSlO1N8t+qtX8CAAD//w==",
				relayState: "iQURykBYIotpOOVTADzkn7WPmpT9DK3tujPKwYKbcVTj84Y4HXSIm2C2",
				sig:        "false signature",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect signed request wrong relay state",
			args{
				request:    "nJJBj9MwEIX/ijX3NJbbpqm1iVS2QlRa2GpTOHCbOgO15NjFMwH236O2i7RIKIe92vP5ved5d4xDONvNKKf4RD9GYlG/hxDZXi4aGHO0CdmzjTgQW3G223x8sGamLTJTFp8ivELO08w5J0kuBVC7bQO+L+a1WRiqjTviqidz7N0a5z2tK1wtK1dXPerFaj6vQH2hzD7FBsxMg9oxj7SLLBilAaONKfSiMPqgV9bUdrGerZfVV1BbYvER5UqeRM62LENyGE6JxS611qa8+C677hHU5m+k+xR5HCh3lH96R5+fHv5D11rrG4yOQe1fsr3zsffx+/RHHG9DbD8cDvti/9gdoL0uw16TZfU+5QFl+pHLie+Lb9dRS1G8PEM74XMgwR4F78pXUu1LCT7hQLvtPgXvnt8gLxkje4oCahNC+nWfCYUakDwSlO1N8t+qtX8CAAD//w==",
				relayState: "wrong relaystate",
				sig:        "Kg1KLmUqSMVliymLBwUq09inVVHNx1UON86C3rmAyKXKj6q0av5qwlZova0htjpGqGcyZTEY4gJSM6FLN+bUjP4DVQul96jUr7+AFw4lMma2RrdzEINtzy8KXEHYbMxTTcDr0Mvnn3D7nmUi9inJNJmh4zJJafmQkhok4/DF0c7+AKizQCRIV35JCWf69XxhZFjMzijoKqWrkOSh9id14KktxSaHUyvVRH4LskzPsIuYeysL9xlrS77r3P8zuaU0EbaESwbTp/q/q7hEq6yH6vg1TXcJCIFPZOqTo0/00UAGie/ExmBp/OebvlHjgJP7g/bF6vK5kGFnxQi4To0Y1A==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect signed request wrong sigAlg",
			args{
				request:    "nJJBj9MwEIX/ijX3NJbbpqm1iVS2QlRa2GpTOHCbOgO15NjFMwH236O2i7RIKIe92vP5ved5d4xDONvNKKf4RD9GYlG/hxDZXi4aGHO0CdmzjTgQW3G223x8sGamLTJTFp8ivELO08w5J0kuBVC7bQO+L+a1WRiqjTviqidz7N0a5z2tK1wtK1dXPerFaj6vQH2hzD7FBsxMg9oxj7SLLBilAaONKfSiMPqgV9bUdrGerZfVV1BbYvER5UqeRM62LENyGE6JxS611qa8+C677hHU5m+k+xR5HCh3lH96R5+fHv5D11rrG4yOQe1fsr3zsffx+/RHHG9DbD8cDvti/9gdoL0uw16TZfU+5QFl+pHLie+Lb9dRS1G8PEM74XMgwR4F78pXUu1LCT7hQLvtPgXvnt8gLxkje4oCahNC+nWfCYUakDwSlO1N8t+qtX8CAAD//w==",
				relayState: "iQURykBYIotpOOVTADzkn7WPmpT9DK3tujPKwYKbcVTj84Y4HXSIm2C2",
				sig:        "Kg1KLmUqSMVliymLBwUq09inVVHNx1UON86C3rmAyKXKj6q0av5qwlZova0htjpGqGcyZTEY4gJSM6FLN+bUjP4DVQul96jUr7+AFw4lMma2RrdzEINtzy8KXEHYbMxTTcDr0Mvnn3D7nmUi9inJNJmh4zJJafmQkhok4/DF0c7+AKizQCRIV35JCWf69XxhZFjMzijoKqWrkOSh9id14KktxSaHUyvVRH4LskzPsIuYeysL9xlrS77r3P8zuaU0EbaESwbTp/q/q7hEq6yH6vg1TXcJCIFPZOqTo0/00UAGie/ExmBp/OebvlHjgJP7g/bF6vK5kGFnxQi4To0Y1A==",
				sigAlg:     dsig.RSASHA256SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect signed request request changed",
			args{
				request:    "changed",
				relayState: "iQURykBYIotpOOVTADzkn7WPmpT9DK3tujPKwYKbcVTj84Y4HXSIm2C2",
				sig:        "Kg1KLmUqSMVliymLBwUq09inVVHNx1UON86C3rmAyKXKj6q0av5qwlZova0htjpGqGcyZTEY4gJSM6FLN+bUjP4DVQul96jUr7+AFw4lMma2RrdzEINtzy8KXEHYbMxTTcDr0Mvnn3D7nmUi9inJNJmh4zJJafmQkhok4/DF0c7+AKizQCRIV35JCWf69XxhZFjMzijoKqWrkOSh9id14KktxSaHUyvVRH4LskzPsIuYeysL9xlrS77r3P8zuaU0EbaESwbTp/q/q7hEq6yH6vg1TXcJCIFPZOqTo0/00UAGie/ExmBp/OebvlHjgJP7g/bF6vK5kGFnxQi4To0Y1A==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect no request",
			args{
				request:    "",
				relayState: "Hv9rftq0AHE47MealTo9m7TCIGhLVedUjmlwyCXLgUepny_c_WOO6f3e",
				sig:        "UE1buXT5lJvUMX5N1baY8OOvoOdsYplqiOdB8VYLUD3CfBt6EHlDta560bnKIovl5/xBsL8hZrMBwZXnzmZ5bNt9RYnSQZNxYXl5t/CnNScbdW4pC8I4gWzTxWmsKCQRBw9JvvpZCKojND1kKT0NMTlOPZHTB+Je8zbR2rNCkY4JePnmOIunOCXvfMpRgMScyFTe/udrLaBQPvVIZ7uE8noGzzANqHAOgS7HqvlLT4jBPd7RO3U/+Vp8mIUH+wkff9iZ/Kp9pambgQ18QJJNTb4By16JtHMqrziSAZX05YXXPyWhdontccZL/kOMHXY1VTaR8vABm/pOaX3GozZEPw==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect no relayState",
			args{
				request:    "nJJBj9MwEIX/ijX3No613WStTaSyFaLSwlabwoHb1JlQS45dPBNg/z1qu0iLhHLgas/nN+/53TOO4WTXkxzjM32fiEX9GkNke75oYMrRJmTPNuJIbMXZbv3x0ZqltshMWXyK8AY5zTOnnCS5FEBtNw34fjHcOHS35a0ZzHDo8XAwrrxb1VgPujZlP1RV3ddY3oH6Qpl9ig2YpQa1ZZ5oG1kwSgNGG7PQNwuj97qypbarellX1VdQG2LxEeVCHkVOtihCchiOicWutNamOO9ddN0TqPUfSw8p8jRS7ij/8I4+Pz/+g6611lcYHYPavXp752Pv47f5IA7XIbYf9vvdYvfU7aG9fIa9OMvqfcojyvwj55NzhpdRS1G8vEA7s+dIgj0K3hdvpNrXEnzCkbabXQrevfyHvGSM7CkKqHUI6edDJhRqQPJEULRXyb+r1v4OAAD//w==",
				relayState: "",
				sig:        "UE1buXT5lJvUMX5N1baY8OOvoOdsYplqiOdB8VYLUD3CfBt6EHlDta560bnKIovl5/xBsL8hZrMBwZXnzmZ5bNt9RYnSQZNxYXl5t/CnNScbdW4pC8I4gWzTxWmsKCQRBw9JvvpZCKojND1kKT0NMTlOPZHTB+Je8zbR2rNCkY4JePnmOIunOCXvfMpRgMScyFTe/udrLaBQPvVIZ7uE8noGzzANqHAOgS7HqvlLT4jBPd7RO3U/+Vp8mIUH+wkff9iZ/Kp9pambgQ18QJJNTb4By16JtHMqrziSAZX05YXXPyWhdontccZL/kOMHXY1VTaR8vABm/pOaX3GozZEPw==",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect no sigAlg",
			args{
				request:    "nJJBj9MwEIX/ijX3No613WStTaSyFaLSwlabwoHb1JlQS45dPBNg/z1qu0iLhHLgas/nN+/53TOO4WTXkxzjM32fiEX9GkNke75oYMrRJmTPNuJIbMXZbv3x0ZqltshMWXyK8AY5zTOnnCS5FEBtNw34fjHcOHS35a0ZzHDo8XAwrrxb1VgPujZlP1RV3ddY3oH6Qpl9ig2YpQa1ZZ5oG1kwSgNGG7PQNwuj97qypbarellX1VdQG2LxEeVCHkVOtihCchiOicWutNamOO9ddN0TqPUfSw8p8jRS7ij/8I4+Pz/+g6611lcYHYPavXp752Pv47f5IA7XIbYf9vvdYvfU7aG9fIa9OMvqfcojyvwj55NzhpdRS1G8vEA7s+dIgj0K3hdvpNrXEnzCkbabXQrevfyHvGSM7CkKqHUI6edDJhRqQPJEULRXyb+r1v4OAAD//w==",
				relayState: "Hv9rftq0AHE47MealTo9m7TCIGhLVedUjmlwyCXLgUepny_c_WOO6f3e",
				sig:        "UE1buXT5lJvUMX5N1baY8OOvoOdsYplqiOdB8VYLUD3CfBt6EHlDta560bnKIovl5/xBsL8hZrMBwZXnzmZ5bNt9RYnSQZNxYXl5t/CnNScbdW4pC8I4gWzTxWmsKCQRBw9JvvpZCKojND1kKT0NMTlOPZHTB+Je8zbR2rNCkY4JePnmOIunOCXvfMpRgMScyFTe/udrLaBQPvVIZ7uE8noGzzANqHAOgS7HqvlLT4jBPd7RO3U/+Vp8mIUH+wkff9iZ/Kp9pambgQ18QJJNTb4By16JtHMqrziSAZX05YXXPyWhdontccZL/kOMHXY1VTaR8vABm/pOaX3GozZEPw==",
				sigAlg:     "",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"redirect no sig",
			args{
				request:    "nJJBj9MwEIX/ijX3No613WStTaSyFaLSwlabwoHb1JlQS45dPBNg/z1qu0iLhHLgas/nN+/53TOO4WTXkxzjM32fiEX9GkNke75oYMrRJmTPNuJIbMXZbv3x0ZqltshMWXyK8AY5zTOnnCS5FEBtNw34fjHcOHS35a0ZzHDo8XAwrrxb1VgPujZlP1RV3ddY3oH6Qpl9ig2YpQa1ZZ5oG1kwSgNGG7PQNwuj97qypbarellX1VdQG2LxEeVCHkVOtihCchiOicWutNamOO9ddN0TqPUfSw8p8jRS7ij/8I4+Pz/+g6611lcYHYPavXp752Pv47f5IA7XIbYf9vvdYvfU7aG9fIa9OMvqfcojyvwj55NzhpdRS1G8vEA7s+dIgj0K3hdvpNrXEnzCkbabXQrevfyHvGSM7CkKqHUI6edDJhRqQPJEULRXyb+r1v4OAAD//w==",
				relayState: "Hv9rftq0AHE47MealTo9m7TCIGhLVedUjmlwyCXLgUepny_c_WOO6f3e",
				sig:        "",
				sigAlg:     dsig.RSASHA1SignatureMethod,
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spConfig := &ServiceProviderConfig{Metadata: tt.args.spMetadata}

			sp, err := NewServiceProvider("test", spConfig, "")
			if err != nil {
				t.Errorf("verifyRedirectSignature() got = %v, wanted to create service provider instance", err)
				return
			}

			requestF := func() string {
				return tt.args.request
			}
			relayStateF := func() string {
				return tt.args.relayState
			}
			sigF := func() string {
				return tt.args.sig
			}
			sigAlgF := func() string {
				return tt.args.sigAlg
			}
			spF := func() *ServiceProvider {
				return sp
			}
			errF := func(err error) {
				if (err != nil) != tt.err {
					t.Errorf("verifyRedirectSignature() got = %v, want %v", err, tt.err)
				}
			}

			gotF := verifyRedirectSignature(requestF, relayStateF, sigF, sigAlgF, spF, errF)
			got := gotF()
			if (got != nil) != tt.err {
				t.Errorf("verifyRedirectSignature() got = %v, want %v", got, tt.err)
				return
			}
		})
	}
}

func TestSSO_signaturePostVerificationNecessary(t *testing.T) {
	type args struct {
		sig         *xml_dsig.SignatureType
		binding     string
		idpMetadata *md.IDPSSODescriptorType
		spMetadata  *md.EntityDescriptorType
	}
	tests := []struct {
		name string
		args args
		res  bool
	}{
		{
			"post signature",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: "test"},
				},
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			true,
		},
		{
			"post AuthnRequestsSigned",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: ""},
				},
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "true",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			true,
		},
		{
			"post WantAuthnRequestsSigned",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: ""},
				},
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "true",
				},
			},
			true,
		},
		{
			"post no signature",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: ""},
				},
				binding: PostBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			false,
		},
		{
			"redirect all required",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: "test"},
				},
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "true",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "true",
				},
			},
			false,
		},
		{
			"redirect nothing required",
			args{
				sig: &xml_dsig.SignatureType{
					SignatureValue: xml_dsig.SignatureValueType{Text: ""},
				},
				binding: RedirectBinding,
				spMetadata: &md.EntityDescriptorType{
					SPSSODescriptor: &md.SPSSODescriptorType{
						AuthnRequestsSigned: "false",
					},
				},
				idpMetadata: &md.IDPSSODescriptorType{
					WantAuthnRequestsSigned: "false",
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idpMetadataF := func() *md.IDPSSODescriptorType {
				return tt.args.idpMetadata
			}
			spMetadataF := func() *md.EntityDescriptorType {
				return tt.args.spMetadata
			}
			signatureF := func() *xml_dsig.SignatureType {
				return tt.args.sig
			}
			bindingF := func() string {
				return tt.args.binding
			}

			gotF := signaturePostVerificationNecessary(idpMetadataF, spMetadataF, signatureF, bindingF)
			got := gotF()
			if got != tt.res {
				t.Errorf("signaturePostVerificationNecessary() got = %v, want %v", got, tt.res)
			}
		})
	}
}

func TestSSO_verifyPostSignature(t *testing.T) {
	type args struct {
		request    string
		spMetadata string
	}
	tests := []struct {
		name string
		args args
		err  bool
	}{
		{
			"post signed request 1",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXNzZXJ0aW9uIiB4bWxuczpzYW1scD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBJRD0iaWQtNzZiYzBiZThiM2JjMmY1YzZjNjhjM2M2YzIxYjU4MWFjNmYzMjg1OSIgVmVyc2lvbj0iMi4wIiBJc3N1ZUluc3RhbnQ9IjIwMjItMDQtMjBUMDg6NTg6NDUuMjY0WiIgRGVzdGluYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6NTAwMDIvc2FtbC9TU08iIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBQcm90b2NvbEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiPjxzYW1sOklzc3VlciBGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OmVudGl0eSI+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGE8L3NhbWw6SXNzdWVyPjxkczpTaWduYXR1cmUgeG1sbnM6ZHM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPjxkczpTaWduZWRJbmZvPjxkczpDYW5vbmljYWxpemF0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jIi8+PGRzOlNpZ25hdHVyZU1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNyc2Etc2hhMSIvPjxkczpSZWZlcmVuY2UgVVJJPSIjaWQtNzZiYzBiZThiM2JjMmY1YzZjNjhjM2M2YzIxYjU4MWFjNmYzMjg1OSI+PGRzOlRyYW5zZm9ybXM+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlIi8+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMTAveG1sLWV4Yy1jMTRuIyIvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI3NoYTEiLz48ZHM6RGlnZXN0VmFsdWU+WTlsa1BvdUFpZFhkVEYyQTQ1UzVra1FwRldVPTwvZHM6RGlnZXN0VmFsdWU+PC9kczpSZWZlcmVuY2U+PC9kczpTaWduZWRJbmZvPjxkczpTaWduYXR1cmVWYWx1ZT5NVDNBT3FITW1YTUZzMXVUN0JxeW9RbDcwSys5QjhzdmFxT010aXpibmFKQUZ6YWQ0d2xNYVpQRE90TmFJem9mMk5lT2lSNDJnM1l4NzdlMms4MFFzSGtNcmVKQnMrRkpXakNBMllaZ29MWmRTM1lYUEw4QlFvT011MU5wNm5CTk02UzZJZXluRlVKT2FEQ2k3TDhmYWlXZTRsOG5lQ1RYSW9iYnR2NzJ6REw5TER6WmVnVDVGMXAxQ0lqUWpHWGRJL0ZhM1NkVldZNE84ZzJwSlBsQXg0bDBUNWlqUjBrY2VkcmdQMTRDN2ZvMktzdi9pdlNsL2d5aUFBeE54ZVR2dlRFZXV2alhGN05VMWtTeGlkUUZoQWFBbnVrd3lCSkNValFiYnBlY3lHZkR1ajduemc5cHh6RXRoZUUvS0JUOTR5c0pzazNud2h1clZaQkxlRG12L1E9PTwvZHM6U2lnbmF0dXJlVmFsdWU+PGRzOktleUluZm8+PGRzOlg1MDlEYXRhPjxkczpYNTA5Q2VydGlmaWNhdGU+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPjxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDp0cmFuc2llbnQiIEFsbG93Q3JlYXRlPSJ0cnVlIi8+PC9zYW1scDpBdXRoblJlcXVlc3Q+",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"post signed request 2",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXNzZXJ0aW9uIiB4bWxuczpzYW1scD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBJRD0iaWQtODQwOGUxZThjY2M4MGM2ZmYxNTc5MmE4NDFhNTY5ZTJkYjQyMmZhZSIgVmVyc2lvbj0iMi4wIiBJc3N1ZUluc3RhbnQ9IjIwMjItMDQtMjBUMDk6MDI6NTIuNzAyWiIgRGVzdGluYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6NTAwMDIvc2FtbC9TU08iIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBQcm90b2NvbEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiPjxzYW1sOklzc3VlciBGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OmVudGl0eSI+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGE8L3NhbWw6SXNzdWVyPjxkczpTaWduYXR1cmUgeG1sbnM6ZHM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPjxkczpTaWduZWRJbmZvPjxkczpDYW5vbmljYWxpemF0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jIi8+PGRzOlNpZ25hdHVyZU1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNyc2Etc2hhMSIvPjxkczpSZWZlcmVuY2UgVVJJPSIjaWQtODQwOGUxZThjY2M4MGM2ZmYxNTc5MmE4NDFhNTY5ZTJkYjQyMmZhZSI+PGRzOlRyYW5zZm9ybXM+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlIi8+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMTAveG1sLWV4Yy1jMTRuIyIvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI3NoYTEiLz48ZHM6RGlnZXN0VmFsdWU+UXpDN1hKa2FvQktkOElxNzV4aSsvdHF5N0NRPTwvZHM6RGlnZXN0VmFsdWU+PC9kczpSZWZlcmVuY2U+PC9kczpTaWduZWRJbmZvPjxkczpTaWduYXR1cmVWYWx1ZT5BQzljdVBZSXNVQmYwais3aHJPakJQL2dielVJSjdnbXV3YVlIS3M2UjlmTTBjbndFN3l5OXo0cjVCUGoxZm9mZHc4Q0l4ZU05U1NYak05N2wyKzJtTGExYldqc0lRcTRzOG45dTRkTmp6bk5INlNXeGpVMGQzOTlpT3BobXBUT2orNXNYS2NmMU1CeEphMFdNZWp1anFKb3BlVE9INjVwUjVuaFJNS0dHY3FsZ1FvS28zTWpNWVFSNHNwVmtlUDFiYW5xblJzT3d1TmhWYTJYcGsyR1FvbUpsSE1iMGNoM2RBazFZNi9UZ3kxcVRxSkJmQTZEUzBSREVLaDVwOHByc0Z1UWdOa2p5QS9SdlNjS0FnOEtHNUJYRkd3SzFCaGZmMTZ3ZDF6SWtna2haSXF2NURSMllrZ29tV0V4YVhXVnZjeWxJUm9QcURBMmRMYmV4dXU0dXc9PTwvZHM6U2lnbmF0dXJlVmFsdWU+PGRzOktleUluZm8+PGRzOlg1MDlEYXRhPjxkczpYNTA5Q2VydGlmaWNhdGU+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPjxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDp0cmFuc2llbnQiIEFsbG93Q3JlYXRlPSJ0cnVlIi8+PC9zYW1scDpBdXRoblJlcXVlc3Q+",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"post signed request 3",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YXNzZXJ0aW9uIiB4bWxuczpzYW1scD0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBJRD0iaWQtYjgxMzE0N2UyMjM2MjU3NWQ4YTZhZjg3NTZlM2RmNWMxMzQxZWEwMCIgVmVyc2lvbj0iMi4wIiBJc3N1ZUluc3RhbnQ9IjIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiIgRGVzdGluYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6NTAwMDIvc2FtbC9TU08iIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBQcm90b2NvbEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiPjxzYW1sOklzc3VlciBGb3JtYXQ9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OmVudGl0eSI+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGE8L3NhbWw6SXNzdWVyPjxkczpTaWduYXR1cmUgeG1sbnM6ZHM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPjxkczpTaWduZWRJbmZvPjxkczpDYW5vbmljYWxpemF0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jIi8+PGRzOlNpZ25hdHVyZU1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNyc2Etc2hhMSIvPjxkczpSZWZlcmVuY2UgVVJJPSIjaWQtYjgxMzE0N2UyMjM2MjU3NWQ4YTZhZjg3NTZlM2RmNWMxMzQxZWEwMCI+PGRzOlRyYW5zZm9ybXM+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlIi8+PGRzOlRyYW5zZm9ybSBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMTAveG1sLWV4Yy1jMTRuIyIvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnI3NoYTEiLz48ZHM6RGlnZXN0VmFsdWU+djhqcXY5Z1J6QXdyYWNrM1BISGl4ZEhPSzVrPTwvZHM6RGlnZXN0VmFsdWU+PC9kczpSZWZlcmVuY2U+PC9kczpTaWduZWRJbmZvPjxkczpTaWduYXR1cmVWYWx1ZT5jSHpWemNHUy84SEVJdlVqUlJGb2M1enh5VVAweVcxc3pELzZ6R1lkYm1NMmh2cDE3RjZ2WE45Rmp0MEpucU5mekFURTBGY3JsK0xYb0RmNTllSHFCdUs3bTRFaWcxMEJIV2ZXa1BTK3dpczVseDZxc2dYcFhoY21VSUlwd2hNbkZveFBIWSs2Z2VDbGlycE02SUluOVd6M1ZGQ0E0by9sRTNqRG1NV1dHYXdTY3gyWHBXaEVFZmVvWnZHWjJ4U1JvL25zM3NTQkYvMXZHQ09KMzZJYzdNbXJyczZLOFZIQ0EvRUdOc0xyODVqMmxQelFqQzYwOTJ1TW9qaVErMDFQUFhqeUJUMkNkZWFvMXhYRU1YSWhvcVFNdmRzZExtcSs2ZGU4c2k0a0ZPVnpiS1Y4QTQ0WldSSEh4eTA1T2xWY1FXWnFoVEZOMUs4eTduRWYxYnp4QlE9PTwvZHM6U2lnbmF0dXJlVmFsdWU+PGRzOktleUluZm8+PGRzOlg1MDlEYXRhPjxkczpYNTA5Q2VydGlmaWNhdGU+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPjxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDp0cmFuc2llbnQiIEFsbG93Q3JlYXRlPSJ0cnVlIi8+PC9zYW1scDpBdXRoblJlcXVlc3Q+",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			false,
		},
		{
			"not base64 encoded",
			args{
				request:    "test not base64",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"not xml but base64 encoded",
			args{
				request:    "dGVzdA==",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"post not signed request",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24geG1sbnM6c2FtbHA9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIElEPWlkLWI4MTMxNDdlMjIzNjI1NzVkOGE2YWY4NzU2ZTNkZjVjMTM0MWVhMDAgVmVyc2lvbj0yLjAgSXNzdWVJbnN0YW50PTIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiBEZXN0aW5hdGlvbj1odHRwOi8vbG9jYWxob3N0OjUwMDAyL3NhbWwvU1NPIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD1odHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9hY3MgUHJvdG9jb2xCaW5kaW5nPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1Q+PHNhbWw6SXNzdWVyIEZvcm1hdD11cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDplbnRpdHk+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGFcPC9zYW1sOklzc3Vlclw+XDxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OnRyYW5zaWVudCBBbGxvd0NyZWF0ZT10cnVlLz48L3NhbWxwOkF1dGhuUmVxdWVzdD4K",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"post signed request wrong certificate",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24geG1sbnM6c2FtbHA9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIElEPWlkLWI4MTMxNDdlMjIzNjI1NzVkOGE2YWY4NzU2ZTNkZjVjMTM0MWVhMDAgVmVyc2lvbj0yLjAgSXNzdWVJbnN0YW50PTIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiBEZXN0aW5hdGlvbj1odHRwOi8vbG9jYWxob3N0OjUwMDAyL3NhbWwvU1NPIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD1odHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9hY3MgUHJvdG9jb2xCaW5kaW5nPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1Q+PHNhbWw6SXNzdWVyIEZvcm1hdD11cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDplbnRpdHk+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGFcPC9zYW1sOklzc3Vlclw+XDxkczpTaWduYXR1cmUgeG1sbnM6ZHM9aHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIz48ZHM6U2lnbmVkSW5mbz48ZHM6Q2Fub25pY2FsaXphdGlvbk1ldGhvZCBBbGdvcml0aG09aHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjcnNhLXNoYTEvPjxkczpSZWZlcmVuY2UgVVJJPSNpZC1iODEzMTQ3ZTIyMzYyNTc1ZDhhNmFmODc1NmUzZGY1YzEzNDFlYTAwPjxkczpUcmFuc2Zvcm1zPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPWh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlLz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjc2hhMS8+PGRzOkRpZ2VzdFZhbHVlPnY4anF2OWdSekF3cmFjazNQSEhpeGRIT0s1az08L2RzOkRpZ2VzdFZhbHVlPjwvZHM6UmVmZXJlbmNlPjwvZHM6U2lnbmVkSW5mbz48ZHM6U2lnbmF0dXJlVmFsdWU+Y0h6VnpjR1MvOEhFSXZValJSRm9jNXp4eVVQMHlXMXN6RC82ekdZZGJtTTJodnAxN0Y2dlhOOUZqdDBKbnFOZnpBVEUwRmNybCtMWG9EZjU5ZUhxQnVLN200RWlnMTBCSFdmV2tQUyt3aXM1bHg2cXNnWHBYaGNtVUlJcHdoTW5Gb3hQSFkrNmdlQ2xpcnBNNklJbjlXejNWRkNBNG8vbEUzakRtTVdXR2F3U2N4MlhwV2hFRWZlb1p2R1oyeFNSby9uczNzU0JGLzF2R0NPSjM2SWM3TW1ycnM2SzhWSENBL0VHTnNMcjg1ajJsUHpRakM2MDkydU1vamlRKzAxUFBYanlCVDJDZGVhbzF4WEVNWElob3FRTXZkc2RMbXErNmRlOHNpNGtGT1Z6YktWOEE0NFpXUkhIeHkwNU9sVmNRV1pxaFRGTjFLOHk3bkVmMWJ6eEJRPT08L2RzOlNpZ25hdHVyZVZhbHVlPjxkczpLZXlJbmZvPjxkczpYNTA5RGF0YT48ZHM6WDUwOUNlcnRpZmljYXRlPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4N3UybmphVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPjxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OnRyYW5zaWVudCBBbGxvd0NyZWF0ZT10cnVlLz48L3NhbWxwOkF1dGhuUmVxdWVzdD4=",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"post signed request wrong signatureValue",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24geG1sbnM6c2FtbHA9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIElEPWlkLWI4MTMxNDdlMjIzNjI1NzVkOGE2YWY4NzU2ZTNkZjVjMTM0MWVhMDAgVmVyc2lvbj0yLjAgSXNzdWVJbnN0YW50PTIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiBEZXN0aW5hdGlvbj1odHRwOi8vbG9jYWxob3N0OjUwMDAyL3NhbWwvU1NPIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD1odHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9hY3MgUHJvdG9jb2xCaW5kaW5nPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1Q+PHNhbWw6SXNzdWVyIEZvcm1hdD11cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDplbnRpdHk+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGFcPC9zYW1sOklzc3Vlclw+XDxkczpTaWduYXR1cmUgeG1sbnM6ZHM9aHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIz48ZHM6U2lnbmVkSW5mbz48ZHM6Q2Fub25pY2FsaXphdGlvbk1ldGhvZCBBbGdvcml0aG09aHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjcnNhLXNoYTEvPjxkczpSZWZlcmVuY2UgVVJJPSNpZC1iODEzMTQ3ZTIyMzYyNTc1ZDhhNmFmODc1NmUzZGY1YzEzNDFlYTAwPjxkczpUcmFuc2Zvcm1zPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPWh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlLz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjc2hhMS8+PGRzOkRpZ2VzdFZhbHVlPnY4anF2OWdSekF3cmFjazNQSEhpeGRIT0s1az08L2RzOkRpZ2VzdFZhbHVlPjwvZHM6UmVmZXJlbmNlPjwvZHM6U2lnbmVkSW5mbz48ZHM6U2lnbmF0dXJlVmFsdWU+Y0h6VnpjR1MvOEhFSXZValJSRm9jNXp4eVVQMDkyanMwYWtucU5mekFURTBGY3JsK0xYb0RmNTllSHFCdUs3bTRFaWcxMEJIV2ZXa1BTK3dpczVseDZxc2dYcFhoY21VSUlwd2hNbkZveFBIWSs2Z2VDbGlycE02SUluOVd6M1ZGQ0E0by9sRTNqRG1NV1dHYXdTY3gyWHBXaEVFZmVvWnZHWjJ4U1JvL25zM3NTQkYvMXZHQ09KMzZJYzdNbXJyczZLOFZIQ0EvRUdOc0xyODVqMmxQelFqQzYwOTJ1TW9qaVErMDFQUFhqeUJUMkNkZWFvMXhYRU1YSWhvcVFNdmRzZExtcSs2ZGU4c2k0a0ZPVnpiS1Y4QTQ0WldSSEh4eTA1T2xWY1FXWnFoVEZOMUs4eTduRWYxYnp4QlE9PTwvZHM6U2lnbmF0dXJlVmFsdWU+PGRzOktleUluZm8+PGRzOlg1MDlEYXRhPjxkczpYNTA5Q2VydGlmaWNhdGU+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvZHM6WDUwOUNlcnRpZmljYXRlPjwvZHM6WDUwOURhdGE+PC9kczpLZXlJbmZvPjwvZHM6U2lnbmF0dXJlPjxzYW1scDpOYW1lSURQb2xpY3kgRm9ybWF0PXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpuYW1laWQtZm9ybWF0OnRyYW5zaWVudCBBbGxvd0NyZWF0ZT10cnVlLz48L3NhbWxwOkF1dGhuUmVxdWVzdD4=",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
		{
			"post signed request wrong digestValue",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24geG1sbnM6c2FtbHA9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIElEPWlkLWI4MTMxNDdlMjIzNjI1NzVkOGE2YWY4NzU2ZTNkZjVjMTM0MWVhMDAgVmVyc2lvbj0yLjAgSXNzdWVJbnN0YW50PTIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiBEZXN0aW5hdGlvbj1odHRwOi8vbG9jYWxob3N0OjUwMDAyL3NhbWwvU1NPIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD1odHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9hY3MgUHJvdG9jb2xCaW5kaW5nPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1Q+PHNhbWw6SXNzdWVyIEZvcm1hdD11cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDplbnRpdHk+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGFcPC9zYW1sOklzc3Vlclw+XDxkczpTaWduYXR1cmUgeG1sbnM6ZHM9aHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIz48ZHM6U2lnbmVkSW5mbz48ZHM6Q2Fub25pY2FsaXphdGlvbk1ldGhvZCBBbGdvcml0aG09aHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjcnNhLXNoYTEvPjxkczpSZWZlcmVuY2UgVVJJPSNpZC1iODEzMTQ3ZTIyMzYyNTc1ZDhhNmFmODc1NmUzZGY1YzEzNDFlYTAwPjxkczpUcmFuc2Zvcm1zPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPWh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlLz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjc2hhMS8+PGRzOkRpZ2VzdFZhbHVlPnY4anF2OWdSemFtMnJhY2szUEhIaXhkSE9LNWs9PC9kczpEaWdlc3RWYWx1ZT48L2RzOlJlZmVyZW5jZT48L2RzOlNpZ25lZEluZm8+PGRzOlNpZ25hdHVyZVZhbHVlPmNIelZ6Y0dTLzhIRUl2VWpSUkZvYzV6eHlVUDB5VzFzekQvNnpHWWRibU0yaHZwMTdGNnZYTjlGanQwSm5xTmZ6QVRFMEZjcmwrTFhvRGY1OWVIcUJ1SzdtNEVpZzEwQkhXZldrUFMrd2lzNWx4NnFzZ1hwWGhjbVVJSXB3aE1uRm94UEhZKzZnZUNsaXJwTTZJSW45V3ozVkZDQTRvL2xFM2pEbU1XV0dhd1NjeDJYcFdoRUVmZW9adkdaMnhTUm8vbnMzc1NCRi8xdkdDT0ozNkljN01tcnJzNks4VkhDQS9FR05zTHI4NWoybFB6UWpDNjA5MnVNb2ppUSswMVBQWGp5QlQyQ2RlYW8xeFhFTVhJaG9xUU12ZHNkTG1xKzZkZThzaTRrRk9WemJLVjhBNDRaV1JISHh5MDVPbFZjUVdacWhURk4xSzh5N25FZjFienhCUT09PC9kczpTaWduYXR1cmVWYWx1ZT48ZHM6S2V5SW5mbz48ZHM6WDUwOURhdGE+PGRzOlg1MDlDZXJ0aWZpY2F0ZT5NSUlDdkRDQ0FhUUNDUUQ2RThaR3NRMnVzakFOQmdrcWhraUc5dzBCQVFzRkFEQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd0hoY05Nakl3TWpFM01UUXdOak01V2hjTk1qTXdNakUzTVRRd05qTTVXakFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDN1hLZENSeFVaWGpkcVZxd3d3T0pxYzFDaDBuT1NtaytVZXJrVXFsdmlXSGRlTFIrRm9sSEtqcUx6Q0Jsb0F6NHhWYzBERmZSNzZnV2NXQUhKbG9xWjdHQlM3TnBEaHpWOEcrY1hRK2JUVTBMdTJlNzN6Q1FiMzBYVWRLaFdpR2ZES2FVKzF4ZzlDRC8yZ0lmc1lQczNUVHExc3E3b0NzNXFMZFVIYVZMNWtjUmFIS2RuVGk3Y3M1aTl4enMzVHNVblhjckpQd3lkanArYUVreVJoMDdvTXBYQkVvYkdpc2ZGMnAxTUE2cFZXMmdqbXl3ZjdENWlZRUZFTFFoTTdwb3FQTjMva2ZCdlUxbjdMZmdxN294bXYvOExGaTRab3ByNW55cXN6MjZYUHRVeTFXcVR6Z3puQW1QK25OMG9CVEVSRlZiWFhkUmEzazJ2NGN4VE5Qbi9BZ01CQUFFd0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFKWXhST1dTT1piT3pYemFmZEdqUUtzTWdOOTQ4Ry9oSHdWdVpuZXlBY1ZvTE1GVHMxV2V5YTlaK3NuTXAxdTBBZERHbVFUUzl6R25EN3N5RFlHT21naWdPTGNNdkxNb1dmNXRDUUJiRXVrVzhPN0RQalJSMFh5cENoR1NzSHNxTEdPMEIwSGFUZWwwSGRQOVNpODI3T0NrYzlRK1dic0ZHLzgvNFRvR1dMK3VsYTFXdUxhd296b2o4dW1QaTlEOGlYQ29XMzV5MlNUVStXRlFHN1crS2ZkdSsyQ1l6LzB0R2R3VnFORzRXc2Zhd1djaHJTMDB2R0ZLam0vZkpjODc2Z0FmeGlNSDFJOWZadllTQXhBWjNzVkkvL01sMnNVZGdmMDY3eXdRNzVvYUxTUzJOSW1tejVhb3MzdnVXbU9YaElMZDdpVFUrQkQ4VXY2dldiSTdJMU09PC9kczpYNTA5Q2VydGlmaWNhdGU+PC9kczpYNTA5RGF0YT48L2RzOktleUluZm8+PC9kczpTaWduYXR1cmU+PHNhbWxwOk5hbWVJRFBvbGljeSBGb3JtYXQ9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOm5hbWVpZC1mb3JtYXQ6dHJhbnNpZW50IEFsbG93Q3JlYXRlPXRydWUvPjwvc2FtbHA6QXV0aG5SZXF1ZXN0Pg==",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},

		{
			"post signed request changed request",
			args{
				request:    "PHNhbWxwOkF1dGhuUmVxdWVzdCB4bWxuczpzYW1sPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDphc3NlcnRpb24geG1sbnM6c2FtbHA9dXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIElEPWlkLWI4MTMxNDdlMjIzNjI1NzVkOGE2YWY4NzU2ZTNkZjVjMTM0MWVhMDAgVmVyc2lvbj0yLjAgSXNzdWVJbnN0YW50PTIwMjItMDQtMjBUMDk6MDM6NTcuNDk1WiBEZXN0aW5hdGlvbj1odHRwOi8vbG9jYWxob3N0OjUwMDAyL3NhbWwvU1NPIEFzc2VydGlvbkNvbnN1bWVyU2VydmljZVVSTD1odHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9hY3MgUHJvdG9jb2xCaW5kaW5nPXVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1Q+PHNhbWw6SXNzdWVyIEZvcm1hdD11cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6bmFtZWlkLWZvcm1hdDplbnRpdHk+aHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGFcPC9zYW1sOklzc3Vlclw+XDxkczpTaWduYXR1cmUgeG1sbnM6ZHM9aHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIz48ZHM6U2lnbmVkSW5mbz48ZHM6Q2Fub25pY2FsaXphdGlvbk1ldGhvZCBBbGdvcml0aG09aHR0cDovL3d3dy53My5vcmcvMjAwMS8xMC94bWwtZXhjLWMxNG4jLz48ZHM6U2lnbmF0dXJlTWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjcnNhLXNoYTEvPjxkczpSZWZlcmVuY2UgVVJJPSNpZC1iODEzMTQ3ZTIyMzYyNTc1ZDhhNmFmODc1NmUzZGY1YzEzNDFlYTAwPjxkczpUcmFuc2Zvcm1zPjxkczpUcmFuc2Zvcm0gQWxnb3JpdGhtPWh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyNlbnZlbG9wZWQtc2lnbmF0dXJlLz48ZHM6VHJhbnNmb3JtIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAxLzEwL3htbC1leGMtYzE0biMvPjwvZHM6VHJhbnNmb3Jtcz48ZHM6RGlnZXN0TWV0aG9kIEFsZ29yaXRobT1odHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjc2hhMS8+PGRzOkRpZ2VzdFZhbHVlPnY4anF2OWdSekF3cmFjazNQSEhpeGRIT0s1az08L2RzOkRpZ2VzdFZhbHVlPjwvZHM6UmVmZXJlbmNlPjwvZHM6U2lnbmVkSW5mbz48ZHM6U2lnbmF0dXJlVmFsdWU+Y0h6VnpjR1MvOEhFSXZValJSRm9jNXp4eVVQMHlXMXN6RC82ekdZZGJtTTJodnAxN0Y2dlhOOUZqdDBKbnFOZnpBVEUwRmNybCtMWG9EZjU5ZUhxQnVLN200RWlnMTBCSFdmV2tQUyt3aXM1bHg2cXNnWHBYaGNtVUlJcHdoTW5Gb3hQSFkrNmdlQ2xpcnBNNklJbjlXejNWRkNBNG8vbEUzakRtTVdXR2F3U2N4MlhwV2hFRWZlb1p2R1oyeFNSby9uczNzU0JGLzF2R0NPSjM2SWM3TW1ycnM2SzhWSENBL0VHTnNMcjg1ajJsUHpRakM2MDkydU1vamlRKzAxUFBYanlCVDJDZGVhbzF4WEVNWElob3FRTXZkc2RMbXErNmRlOHNpNGtGT1Z6YktWOEE0NFpXUkhIeHkwNU9sVmNRV1pxaFRGTjFLOHk3bkVmMWJ6eEJRPT08L2RzOlNpZ25hdHVyZVZhbHVlPjxkczpLZXlJbmZvPjxkczpYNTA5RGF0YT48ZHM6WDUwOUNlcnRpZmljYXRlPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L2RzOlg1MDlDZXJ0aWZpY2F0ZT48L2RzOlg1MDlEYXRhPjwvZHM6S2V5SW5mbz48L2RzOlNpZ25hdHVyZT48c2FtbHA6TmFtZUlEUG9saWN5IEZvcm1hdD1jaGFuZ2VkIEFsbG93Q3JlYXRlPXRydWUvPjwvc2FtbHA6QXV0aG5SZXF1ZXN0Pg==",
				spMetadata: "PEVudGl0eURlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjdaIiBlbnRpdHlJRD0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvbWV0YWRhdGEiPgogIDxTUFNTT0Rlc2NyaXB0b3IgeG1sbnM9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDptZXRhZGF0YSIgdmFsaWRVbnRpbD0iMjAyMi0wNC0xMFQxMzo1MTowNS45NjcyMzhaIiBwcm90b2NvbFN1cHBvcnRFbnVtZXJhdGlvbj0idXJuOm9hc2lzOm5hbWVzOnRjOlNBTUw6Mi4wOnByb3RvY29sIiBBdXRoblJlcXVlc3RzU2lnbmVkPSJ0cnVlIiBXYW50QXNzZXJ0aW9uc1NpZ25lZD0idHJ1ZSI+CiAgICA8S2V5RGVzY3JpcHRvciB1c2U9ImVuY3J5cHRpb24iPgogICAgICA8S2V5SW5mbyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+CiAgICAgICAgPFg1MDlEYXRhIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICAgIDxYNTA5Q2VydGlmaWNhdGUgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPk1JSUN2RENDQWFRQ0NRRDZFOFpHc1EydXNqQU5CZ2txaGtpRzl3MEJBUXNGQURBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3SGhjTk1qSXdNakUzTVRRd05qTTVXaGNOTWpNd01qRTNNVFF3TmpNNVdqQWdNUjR3SEFZRFZRUUREQlZ0ZVhObGNuWnBZMlV1WlhoaGJYQnNaUzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3Z2dFS0FvSUJBUUM3WEtkQ1J4VVpYamRxVnF3d3dPSnFjMUNoMG5PU21rK1VlcmtVcWx2aVdIZGVMUitGb2xIS2pxTHpDQmxvQXo0eFZjMERGZlI3NmdXY1dBSEpsb3FaN0dCUzdOcERoelY4RytjWFErYlRVMEx1MmU3M3pDUWIzMFhVZEtoV2lHZkRLYVUrMXhnOUNELzJnSWZzWVBzM1RUcTFzcTdvQ3M1cUxkVUhhVkw1a2NSYUhLZG5UaTdjczVpOXh6czNUc1VuWGNySlB3eWRqcCthRWt5UmgwN29NcFhCRW9iR2lzZkYycDFNQTZwVlcyZ2pteXdmN0Q1aVlFRkVMUWhNN3BvcVBOMy9rZkJ2VTFuN0xmZ3E3b3htdi84TEZpNFpvcHI1bnlxc3oyNlhQdFV5MVdxVHpnem5BbVArbk4wb0JURVJGVmJYWGRSYTNrMnY0Y3hUTlBuL0FnTUJBQUV3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUpZeFJPV1NPWmJPelh6YWZkR2pRS3NNZ045NDhHL2hId1Z1Wm5leUFjVm9MTUZUczFXZXlhOVorc25NcDF1MEFkREdtUVRTOXpHbkQ3c3lEWUdPbWdpZ09MY012TE1vV2Y1dENRQmJFdWtXOE83RFBqUlIwWHlwQ2hHU3NIc3FMR08wQjBIYVRlbDBIZFA5U2k4MjdPQ2tjOVErV2JzRkcvOC80VG9HV0wrdWxhMVd1TGF3b3pvajh1bVBpOUQ4aVhDb1czNXkyU1RVK1dGUUc3VytLZmR1KzJDWXovMHRHZHdWcU5HNFdzZmF3V2NoclMwMHZHRktqbS9mSmM4NzZnQWZ4aU1IMUk5Zlp2WVNBeEFaM3NWSS8vTWwyc1VkZ2YwNjd5d1E3NW9hTFNTMk5JbW16NWFvczN2dVdtT1hoSUxkN2lUVStCRDhVdjZ2V2JJN0kxTT08L1g1MDlDZXJ0aWZpY2F0ZT4KICAgICAgICA8L1g1MDlEYXRhPgogICAgICA8L0tleUluZm8+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjYWVzMTI4LWNiYyI+PC9FbmNyeXB0aW9uTWV0aG9kPgogICAgICA8RW5jcnlwdGlvbk1ldGhvZCBBbGdvcml0aG09Imh0dHA6Ly93d3cudzMub3JnLzIwMDEvMDQveG1sZW5jI2FlczE5Mi1jYmMiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgICAgPEVuY3J5cHRpb25NZXRob2QgQWxnb3JpdGhtPSJodHRwOi8vd3d3LnczLm9yZy8yMDAxLzA0L3htbGVuYyNhZXMyNTYtY2JjIj48L0VuY3J5cHRpb25NZXRob2Q+CiAgICAgIDxFbmNyeXB0aW9uTWV0aG9kIEFsZ29yaXRobT0iaHR0cDovL3d3dy53My5vcmcvMjAwMS8wNC94bWxlbmMjcnNhLW9hZXAtbWdmMXAiPjwvRW5jcnlwdGlvbk1ldGhvZD4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxLZXlEZXNjcmlwdG9yIHVzZT0ic2lnbmluZyI+CiAgICAgIDxLZXlJbmZvIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwLzA5L3htbGRzaWcjIj4KICAgICAgICA8WDUwOURhdGEgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvMDkveG1sZHNpZyMiPgogICAgICAgICAgPFg1MDlDZXJ0aWZpY2F0ZSB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC8wOS94bWxkc2lnIyI+TUlJQ3ZEQ0NBYVFDQ1FENkU4WkdzUTJ1c2pBTkJna3Foa2lHOXcwQkFRc0ZBREFnTVI0d0hBWURWUVFEREJWdGVYTmxjblpwWTJVdVpYaGhiWEJzWlM1amIyMHdIaGNOTWpJd01qRTNNVFF3TmpNNVdoY05Nak13TWpFM01UUXdOak01V2pBZ01SNHdIQVlEVlFRRERCVnRlWE5sY25acFkyVXVaWGhoYlhCc1pTNWpiMjB3Z2dFaU1BMEdDU3FHU0liM0RRRUJBUVVBQTRJQkR3QXdnZ0VLQW9JQkFRQzdYS2RDUnhVWlhqZHFWcXd3d09KcWMxQ2gwbk9TbWsrVWVya1VxbHZpV0hkZUxSK0ZvbEhLanFMekNCbG9BejR4VmMwREZmUjc2Z1djV0FISmxvcVo3R0JTN05wRGh6VjhHK2NYUStiVFUwTHUyZTczekNRYjMwWFVkS2hXaUdmREthVSsxeGc5Q0QvMmdJZnNZUHMzVFRxMXNxN29DczVxTGRVSGFWTDVrY1JhSEtkblRpN2NzNWk5eHpzM1RzVW5YY3JKUHd5ZGpwK2FFa3lSaDA3b01wWEJFb2JHaXNmRjJwMU1BNnBWVzJnam15d2Y3RDVpWUVGRUxRaE03cG9xUE4zL2tmQnZVMW43TGZncTdveG12LzhMRmk0Wm9wcjVueXFzejI2WFB0VXkxV3FUemd6bkFtUCtuTjBvQlRFUkZWYlhYZFJhM2sydjRjeFROUG4vQWdNQkFBRXdEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSll4Uk9XU09aYk96WHphZmRHalFLc01nTjk0OEcvaEh3VnVabmV5QWNWb0xNRlRzMVdleWE5Witzbk1wMXUwQWRER21RVFM5ekduRDdzeURZR09tZ2lnT0xjTXZMTW9XZjV0Q1FCYkV1a1c4TzdEUGpSUjBYeXBDaEdTc0hzcUxHTzBCMEhhVGVsMEhkUDlTaTgyN09Da2M5UStXYnNGRy84LzRUb0dXTCt1bGExV3VMYXdvem9qOHVtUGk5RDhpWENvVzM1eTJTVFUrV0ZRRzdXK0tmZHUrMkNZei8wdEdkd1ZxTkc0V3NmYXdXY2hyUzAwdkdGS2ptL2ZKYzg3NmdBZnhpTUgxSTlmWnZZU0F4QVozc1ZJLy9NbDJzVWRnZjA2N3l3UTc1b2FMU1MyTkltbXo1YW9zM3Z1V21PWGhJTGQ3aVRVK0JEOFV2NnZXYkk3STFNPTwvWDUwOUNlcnRpZmljYXRlPgogICAgICAgIDwvWDUwOURhdGE+CiAgICAgIDwvS2V5SW5mbz4KICAgIDwvS2V5RGVzY3JpcHRvcj4KICAgIDxTaW5nbGVMb2dvdXRTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLVBPU1QiIExvY2F0aW9uPSJodHRwOi8vbG9jYWxob3N0OjgwMDAvc2FtbC9zbG8iIFJlc3BvbnNlTG9jYXRpb249Imh0dHA6Ly9sb2NhbGhvc3Q6ODAwMC9zYW1sL3NsbyI+PC9TaW5nbGVMb2dvdXRTZXJ2aWNlPgogICAgPEFzc2VydGlvbkNvbnN1bWVyU2VydmljZSBCaW5kaW5nPSJ1cm46b2FzaXM6bmFtZXM6dGM6U0FNTDoyLjA6YmluZGluZ3M6SFRUUC1QT1NUIiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMSI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgICA8QXNzZXJ0aW9uQ29uc3VtZXJTZXJ2aWNlIEJpbmRpbmc9InVybjpvYXNpczpuYW1lczp0YzpTQU1MOjIuMDpiaW5kaW5nczpIVFRQLUFydGlmYWN0IiBMb2NhdGlvbj0iaHR0cDovL2xvY2FsaG9zdDo4MDAwL3NhbWwvYWNzIiBpbmRleD0iMiI+PC9Bc3NlcnRpb25Db25zdW1lclNlcnZpY2U+CiAgPC9TUFNTT0Rlc2NyaXB0b3I+CjwvRW50aXR5RGVzY3JpcHRvcj4=",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spConfig := &ServiceProviderConfig{Metadata: tt.args.spMetadata}

			sp, err := NewServiceProvider("test", spConfig, "")
			if err != nil {
				t.Errorf("verifyPostSignature() got = %v, wanted to create service provider instance", err)
				return
			}

			requestF := func() string {
				return tt.args.request
			}
			spF := func() *ServiceProvider {
				return sp
			}
			errF := func(err error) {
				if (err != nil) != tt.err {
					t.Errorf("verifyPostSignature() got = %v, want %v", err, tt.err)
				}
			}

			gotF := verifyPostSignature(requestF, spF, errF)
			got := gotF()
			if (got != nil) != tt.err {
				t.Errorf("verifyPostSignature() got = %v, want %v", got, tt.err)
				return
			}
		})
	}
}
