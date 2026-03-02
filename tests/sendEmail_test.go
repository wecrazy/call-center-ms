package tests

import (
	"call_center_app/whatsapp"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// go test -v -timeout 60m ./tests/sendEmail_test.go
func TestSendEmail(t *testing.T) {
	to := []string{
		"wegirandol@smartwebindonesia.com",
	}

	cc := []string{
		"lanitahinari570@gmail.com",
	}

	mjmlTemplate := fmt.Sprintf(`
		<mjml>
		<mj-head>
			<mj-preview>Follow-up Report by Team Call Center</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>

		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear All,</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
				We would like to attach the report about the count of results followed up by the team call center per %v.
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%v</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>Call Center Team</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		</mjml>
		`,
		time.Now().Format("02 January 2006"),
		"PT. Cyber Smart Network Asia",
		"08123456789",
	)

	attachment := []whatsapp.EmailAttachment{}
	err := whatsapp.SendMail(to, cc, "test kirim email", mjmlTemplate, attachment)
	t.Log(err)
	assert.NoError(t, err, "SendMail should not return an error")
}
