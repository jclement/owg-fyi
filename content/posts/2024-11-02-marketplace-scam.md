---
title: Marketplace Scam
date: 2024-11-02
---

# Marketplace Scam

We recently tried to sell a "well-loved" baritone saxophone case on Facebook Marketplace. To our surprise, we had almost immediate interest! Who knew that an item like this would be so popular?

> Hi, is it in good condition? Best regards, Jeremy Gabriel

Well... Good-ish...

![Does this look 'good' to you, Jeremy?](/media/marketplace-scam/image-23.png)

Hmmm. That's odd. Immediately, this sounds a bit suspicious. The item is listed as "Used - Fair Condition," the description elaborates on that with "This well-loved case has been on a few adventures, showing some wear and tear." Perhaps Jeremy was just so excited to finally get his hands on the elusive bari case that he skipped reading the listing or looking at the pictures. Oh well... "Jeremy, it's well used but functional."

> Now I am away from the city, but I will be able to pick it up on November 6rd. Can you reserve it for me? I can make payment in advance to avoid any delays. Could you please provide me with your information for electronic transfer?

Hmmm. Wow. This dude is eager, but that's ok; we're eager to sell this thing. Also, 6rd?

However, my scam-dar goes to 11 when he says, "I can make payment in advance to avoid any delays"... At this point, I'm 95% sure this is a scam, but I'm interested to see where this goes. "Sure, Jeremy, I'll hold it for you for $50.", I say.

> I sent through interac, go to your e-mail and select the bank where the funds will arrive

Well. How helpful he is in guiding me to log in to my bank! What a pal!

Sure enough, I got this surprisingly realistic-looking email notification from Mr. Jeremy Gabriel. Sweet. Selling sax cases is easy!

![](/media/marketplace-scam/signal-2024-11-02-172945-002.jpeg)
![](/media/marketplace-scam/fastmail.jpg)

But... that from address looks sketchy. Offhand, I'm not sure I know what domain emails from Interac use, but I doubt it's "paymant.japanesefilms.org." Note that while it was fairly obvious from my desktop mail client, the address was hidden on the iPhone (although you can click on the sender's name to see the email address).

Ok. Yup. Definitely a scam. Too bad, we really wanted to sell this case.

> **Note:** I took these steps from a laptop running QubesOS using a throw-away VM connected to the TOR network to limit the chances of this thing doing anything malicious to me. It should go without saying, but don't open suspect emails because you are curious!

So, let's see where these links take us!

Well. First of all, I had to get past a Cloudflare captcha, probably because I was coming at this thing from Tor. Cloudflare is super handy, but it's annoying to see it used for crap like this.

I then got this bonus challenge. At this point, I'm thinking to myself, "This must be the real thing. Look how secure it is!" (not)

![Picture? They couldn't even be bothered to generate a picture with numbers. It's just text.](/media/marketplace-scam/screenshot-2024-11-02-17-00-04.png)

And then I'm into the surprisingly legit-looking Interac landing page.

> **Note:** For those of you not in Canada, Interac e-transfers are a common way to send money between individual in Canada. Loosely like Venmo in the U.S.

![It looks almost real!](/media/marketplace-scam/signal-2024-11-02-171333-002.png)

Wow. This is really well done. Aside from the obviously sketchy domain, this thing looks pretty legit and even has a friendly lady in the bottom right to answer my questions (I tried, she ignored me...). Again, noticing the bogus domain would have been harder if I had done this from a phone.

Clicking on any of those banks brought me to very realistic (again, aside from the domain) bank login pages. I tried out RBC, ATB, and TD, all of which looked pretty good.

![](/media/marketplace-scam/signal-2024-11-02-171333-006.jpeg)
![](/media/marketplace-scam/signal-2024-11-02-171333-005.png)
![](/media/marketplace-scam/signal-2024-11-02-171333-003.png)

Trying to log in to any of these banks (**with fake credentials)** posted those credentials back to the site via a websocket and brought up a spinner like this. (Note that when the scammers had to write their own content on these pages, the quality of the grammar dropped noticeably - "The bank processes your data").

![If I had entered real credentials, I would have been sad right about now...](/media/marketplace-scam/signal-2024-11-02-171333-004-1.png)

I assume that had I entered real credentials, they would have tried to log in using those credentials and then prompted me for the 2FA code. In fact, the code contains a list of text responses which I'd assume they use to prompt their victims as they attempt to log in to their accounts.
```json
  "app_code_title": "Within 2 minutes, the verification code will be sent to your banking application.",
  "app_code_text": "Enter the code that was sent to your banking application",
  "call_code_title": "The bank will give you a verification code over the phone",
  "call_code_text": "Enter the code the bank gave you over the phone",
```
The full list of strings in this file is pretty interesting:
```json
{
  "information": "Information",
  "recipient": "The recipient of the package",
  "delivery_address": "Delivery address",
  "phone_number": "Phone number",
  "enter_full_address": "Enter the full address",
  "enter_full_name": "Enter the full name",
  "enter_phone_number": "Enter your phone number",
  "product_name": "Product name",
  "track_number": "Departure track number",
  "request_payment": "Request for payment",
  "tracking": "Tracking",
  "shipment": "Shipment",
  "shipment_created": "A shipment was created",
  "waiting_payment": "Waiting for payment",
  "package_paid": "The package is paid",
  "package_delivered": "The package was delivered to the courier",
  "package_delivered_2": "The package was delivered to the recipient",
  "payment": "Payment",
  "details": "Details",
  "certified1": "Certified by authorized partners",
  "certified2": "Free 1-month warranty and 7-day return.",
  "information_delivery": "Delivery information",
  "price": "Price",
  "new_good": "New/used",
  "delivery": "Delivery",
  "state": "Condition",
  "amount": "Quantity",
  "product_waiting_for_payment": "The product is waiting for payment!",
  "continue": "CONTINUE",
  "payments_safe": "Making payments is safe",
  "agreement": "By clicking the \"Pay\" button you accept the terms of the User Agreement on the \"Secure Transaction\" Online Service",
  "about_delivery": "About delivery",
  "about_delivery_1": "After payment, your money will be reserved by the system until you receive the package.",
  "about_delivery_2": "The courier will contact you and indicate on what day and time he can deliver the\nparcel to the specified address.",
  "about_delivery_3": "Courier service is fast, convenient and safe!\nAll our couriers undergo additional medical examinations.",
  "card_holder": "Card Holder",
  "full_name": "Full Name",
  "other_bank": "Another bank",
  "expires": "Expires",
  "card_number": "Card Number",
  "expiration_date": "Expiration Date",
  "month": "Month",
  "year": "Year",
  "submit": "Submit",
  "card_invalid": "Card number invalid",
  "support_chat": "Support Chat",
  "enter_message": "Enter a message...",
  "send": "Send",
  "you": "You",
  "support": "Support",
  "another_card": "At this moment we do not cooperate with cards of this bank. You need to specify a card of another bank!",
  "limits": "You need to raise the internet limits on your card to make a transaction!",
  "success": "Success",
  "payment_completed": "Payment has been successfully completed",
  "money_will": "The money will be transferred to your bank card within 24 hours.",
  "push_title": "To verify your card, confirm the transaction on the bank's mobile app",
  "push_text": "To transfer money to your card, your card must be verified. This is a request to verify the card, but you will not be charged, the system will simply state that the card is not blocked and is ready for the transaction.",
  "sms_code_title": "Enter SMS code",
  "sms_code_text": "A one-time SMS code has been sent to your phone",
  "app_code_title": "Within 2 minutes, the verification code will be sent to your banking application.",
  "app_code_text": "Enter the code that was sent to your banking application",
  "call_code_title": "The bank will give you a verification code over the phone",
  "call_code_text": "Enter the code the bank gave you over the phone",
  "blik_code_title": "BLIK payment",
  "blik_code_text": "Generate a Blik code on your account and enter it",
  "bank_process": "The bank processes your data. This may take some time, do not close the page",
  "wait": "Wait a little while",
  "enter_code": "Enter the code",
  "try_again": "Try again",
  "wrong_code": "You entered the wrong code",
  "select_your_bank": "Select your bank",
  "another_bank": "Another bank",
  "choice_bank": "Choice of bank",
  "online_bank_login": "Online bank login",
  "username": "Username",
  "password": "Password",
  "birthday": "Date of Birthday",
  "login": "Log in",
  "fake.title": "Receipt of payment from the buyer",
  "fake.product_payed": "Your product has been paid",
  "fake.client_payed": "The customer has already paid for the order",
  "fake.delivery_info": "Shipping information",
  "fake.address": "Delivery address",
  "fake.full_name": "Full name",
  "fake.instruction": "Instructions for receiving money. After receiving the money, you must either send the goods to the buyer or hand them over to a courier who will call you within 12 hours.",
  "fake.instruction2": "Please provide the buyer's delivery number after shipping! Goods must be shipped within 3 days of receipt of money.",
  "fake.instruction3": "By clicking on the \"Receive Money\" button, you accept the terms of the user agreement with the \"Safe Offer\" online service.",
  "fake.receive_money": "Receive Money",
  "fake.operations_safe": "Transactions made through our service are completely secure. Unique data encryption system",
  "fake2.title": "Payment for the product",
  "fake2.product_payed": "Your item is registered!",
  "fake2.client_payed": "The seller is waiting for the payment of the order.",
  "fake2.instruction": "After the payment of the product, the seller sends the product to the buyer within the specified period or hands over the product to the courier, who will call you back within 12 hours.",
  "fake2.instruction2": "After the shipment of the product, the seller must provide the buyer with the shipping number! The shipment of the product must be within within 3 days after receipt of the money",
  "fake2.instruction3": "By clicking the \"PAY\" button, you agree to the following terms Terms of Use Agreement by using the online service \"Secure Offer\".",
  "fake2.receive_money": "PAY",
  "fill_the_input": "Fill in the input field",
  "attention": "Attention",
  "bank_requested": "The bank requested additional bank card information to verify card ownership.",
  "enter_exact_balance": "Enter the exact balance of your card",
  "balance_number": "The balance of the card must be a number",
  "wu.title": "Receive Money Transfers",
  "wu.desc": "Receive money through Western Union via cash pickup at an agent location. Choose the way that is best for you to get started.",
  "wu.sendMoney": "Send money",
  "wu.trackTransfer": "Track a transfer",
  "wu.findLocations": "Find locations",
  "wu.help": "Help",
  "wu.receiveMoney": "Receive money",
  "wu.receiveMoneyDesc": "Western Union offers many convenient ways for you to receive money transfers from abroad or in Canada. You and your sender can choose what’s best for you.",
  "wu.receiveTo": "Receive to",
  "wu.receiverName": "Receiver name",
  "wu.toReceiveMoney": "To receive money, enter your 10 digits tracking number (MTCN):",
  "wu.receive": "Receive",
  "wu.beInformed": "Be informed. Be aware.",
  "wu.protectYourself": "Protect yourself from fraud",
  "wu.referFriend": "Refer a friend",
  "wu.refer1": "You’ll both earn a $10 gift card redeemable at",
  "wu.refer2": "Amazon, John Lewis, M&S and more.",
  "wu.refer3": "Terms and conditions apply.",
  "wu.learnMore": "Learn more",
  "wu.receiveWays": "Receive your money transfer in a variety of ways",
  "wu.receiveWays2": "Knowing money is on its way is a good feeling. Whether you need money because your flight was cancelled, for an emergency repair, or you’re simply the recipient of a very nice birthday gift, Western Union makes it easy to receive money in a variety of ways. Whatever the occasion, receiving money from abroad or in Canada. is not only a lifesaver — it’s a reminder that the world isn’t nearly as big as it felt a few minutes ago.",
  "wu.receiveWays3": "Receive money to your bank account",
  "wu.receiveWays4": "Your loved ones can send money directly to billions of bank accounts. Check if you've received the money transfer using digital or online banking, online at westernunion.com, or with the Western Union® app.",
  "wu.receiveWays5": "Receive money transfers directly to your mobile wallet",
  "wu.receiveWays6": "In selected countries, you can receive money directly to your mobile wallet",
  "wu.trackReceive": "When you receive money, track your transfer on our app",
  "wu.trackReceive2": "Send money on the go or start a transfer and pay in-store.",
  "wu.trackReceive3": "Track your money transfer in real time.",
  "wu.trackReceive4": "Repeat transfers quickly to friends and family.",
  "wu.faq": "Frequently asked questions about receiving money from abroad",
  "wu.faq2": "How will I know if my money is ready to collect?",
  "wu.faq3": "How do I receive money with Western Union without a bank account?",
  "wu.faq4": "Still have questions about sending money online?",
  "wu.faq5": "Contact our Customer Care team or visit our FAQ page for more information.",
  "wu.contact": "Contact",
  "wu.info1": "Funds may be delayed or services unavailable based on certain transaction conditions, including amount sent, destination country, currency availability, regulatory issues, identification requirements, Agent location hours, differences in time zones, or selection of delayed options. For mobile transactions funds will be paid to receiver’s mWallet account provider for credit to account tied to receiver’s mobile number. Additional third-party charges may apply, including SMS and account over-limit and cash-out fees. See the transfer form for restrictions.",
  "wu.info2": "Do not share details of the money transfer to anyone other than your receiver.",
  "wu.info3": "Western Union also makes money from currency exchange. When choosing a money transmitter, carefully compare both transfer fees and exchange rates. Fees, foreign exchange rates and taxes may vary by brand, channel, and location based on a number of factors. Fees and rates subject to change without notice.",
  "wu.footer.nav1": "Money Transfer",
  "wu.footer.nav1.1": "Send money",
  "wu.footer.nav1.2": "Send money online",
  "wu.footer.nav1.3": "Send money in person",
  "wu.footer.nav1.4": "Track a transfer",
  "wu.footer.nav1.5": "Receive money",
  "wu.footer.nav1.6": "Find locations",
  "wu.footer.nav1.7": "Money transfer app",
  "wu.footer.nav1.8": "Currency converter",
  "wu.footer.nav2": "Company",
  "wu.footer.nav2.1": "About us",
  "wu.footer.nav2.2": "Contact us",
  "wu.footer.nav2.3": "FAQ",
  "wu.footer.nav2.4": "Blog",
  "wu.footer.nav2.5": "Careers",
  "wu.footer.nav2.6": "Investor relationships",
  "wu.footer.nav2.7": "WU foundation",
  "wu.footer.nav3": "Quick Links",
  "wu.footer.nav3.1": "Log in / Register",
  "wu.footer.nav3.2": "Refer a Friend",
  "wu.footer.nav3.3": "Become an agent",
  "wu.footer.nav3.4": "WU Business Solutions",
  "wu.footer.nav3.5": "Fraud Awareness",
  "wu.footer.nav3.6": "Individual Rights Request",
  "wu.footer.nav4": "Legal",
  "wu.footer.nav4.1": "Terms & Conditions",
  "wu.footer.nav4.2": "Intellectual",
  "wu.footer.nav4.3": "Online Privacy Statement",
  "wu.footer.nav4.4": "Current Modern Slavery Statement",
  "wu.footer.nav4.5": "Historical Modern Slavery Statement",
  "wu.footer.info": "Send money online to 200 countries and territories with hundreds of thousands of Western Union agent locations.",
  "wu.footer.links1": "Home",
  "wu.footer.links2": "Corporate Info",
  "wu.footer.links3": "About us",
  "wu.footer.links4": "Contact us",
  "wu.footer.links5": "Fraud awareness",
  "wu.footer.links6": "Online Privacy Statement",
  "wu.footer.links7": "Terms & Conditions",
  "wu.footer.links8": "Cookie information",
  "wu.footer.sublink.1": "Blog",
  "wu.footer.sublink.2": "Careers",
  "wu.footer.sublink.3": "Become an agent",
  "wu.footer.sublink.4": "My WU",
  "wu.footer.sublink.5": "WU Foundation",
  "wu.footer.sublink.6": "Report a security Bug",
  "wu.footer.sublink.7": "Investor relationship",
  "wu.footer.sublink.8": "Intellectual property",
  "wu.footer.sublink.9": "Sitemap",
  "wu.footer.info2": "© 2021 Western Union Holdings, Inc. All Rights Reserved. All logos, trademarks, service marks, and trade names referenced in this material are the property of their respective owners.",
  "wu.footer.info3": "The Western Union® Online Service is offered in Canada by Western Union International Bank GmbH, UK Branch (WUIB) in cooperation with Western Union International Limited. WUIB, trading as Western Union International Bank, is authorised and regulated by the Austrian Financial Market Authority and also subject to regulation by the Financial Conduct Authority and limited regulation by the Prudential Regulation Authority.",
  "country_name": "Canada",
  "refund_1": "To get your money back fill out a refund form:",
  "refund_2": "Issue a refund",
  "bl.title": "Billing Info",
  "bl.name": "Name",
  "bl.address": "Address",
  "bl.address2": "Address 2",
  "bl.city": "City",
  "bl.state": "State",
  "bl.zip": "Zip / Postal Code",
  "bl.country": "Country",
  "exact_balance": "You need to indicate your exact balance to pass the bank verification data.\nWe need to make sure that your account has not been compromised, that your card has not been lost or stolen and that only you know your balance.\nPlease enter the exact amount as $999.99."
}
```
Ugh...

> You need to raise the internet limits on your card to make a transaction!

Double Ugh...

> The bank requested additional bank card information to verify card ownership.

One last thing that jumped out from the page source. These pages were realistic because they were just copies of the real pages with images inlined through a tool called [Single File](https://chromewebstore.google.com/detail/singlefile/mpiodijhokgodhhofbcjdecpffjipkle?ref=straybits.ca). Not that I have the means to attribute this correctly, but the "(Москва, стандартное время)" in the timestamp in the embedded comment says "Moscow Standard Time."
```html
<!DOCTYPE html>
<html class="fa-events-icons-ready">
<!--
 Page saved with SingleFile 
 url: https://identity.auth.atb.com/login........ 
 saved date: Tue Oct 17 2023 13:55:34 GMT+0300 (Москва, стандартное время)
-->
```
The nutshell from this... The bad guys are seriously upping their game, and this scam is insidious.

We're all told not to click on links in emails unless it's something you expect from someone you trust. However, were this a real buyer, I would also have received an email from Interac (presumably not at `japanesefilms.org`) and would have needed to click that link and log into my bank... I do this all the time, and I could easily see falling for this, unfortunately. Heck, I could easily see *myself* falling for this if I were rushing around and using my phone.

The moral of the story? Always practice ‘safe sax’ on Facebook Marketplace because scammers are blowing a different tune!

> *I'm so sorry for that "safe sax" comment... that was dreadful...  
> - Jeff*

BTW, if you are looking for a well-loved bari case, please get in touch ;)

> **Note:** If you encounter something similar you should report it to Facebook (instructions).

---

*Originally published at [onewheelgeek.me](https://onewheelgeek.me/posts/marketplace-scam/).*
