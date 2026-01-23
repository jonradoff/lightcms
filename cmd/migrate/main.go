package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"lightcms/internal/dbutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Content represents a content item
type Content struct {
	ID              primitive.ObjectID     `bson:"_id,omitempty"`
	TemplateID      primitive.ObjectID     `bson:"template_id"`
	TemplateName    string                 `bson:"template_name"`
	Title           string                 `bson:"title"`
	Slug            string                 `bson:"slug"`
	FullPath        string                 `bson:"full_path"`
	Category        string                 `bson:"category"`
	MetaDescription string                 `bson:"meta_description"`
	OGImage         string                 `bson:"og_image"`
	Data            map[string]interface{} `bson:"data"`
	Published       bool                   `bson:"published"`
	PublishedAt     *time.Time             `bson:"published_at,omitempty"`
	UseHeader       bool                   `bson:"use_header"`
	UseFooter       bool                   `bson:"use_footer"`
	UseTheme        bool                   `bson:"use_theme"`
	RawMode         bool                   `bson:"raw_mode"`
	CreatedAt       time.Time              `bson:"created_at"`
	UpdatedAt       time.Time              `bson:"updated_at"`
}

// Template represents a template
type Template struct {
	ID   primitive.ObjectID `bson:"_id"`
	Name string             `bson:"name"`
	Slug string             `bson:"slug"`
}

// PageContent holds the content for a page to be created
type PageContent struct {
	Title           string
	Slug            string
	TemplateSlug    string
	MetaDescription string
	Data            map[string]interface{}
}

func main() {
	// Check for -purge flag
	purge := false
	for _, arg := range os.Args[1:] {
		if arg == "-purge" {
			purge = true
		}
	}

	mongoURI := dbutil.GetMongoURI()
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set. Set it via environment variable or config file.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")

	// Purge all content if requested
	if purge {
		result, err := db.Collection("content").DeleteMany(ctx, bson.M{})
		if err != nil {
			log.Fatal("Failed to purge content:", err)
		}
		fmt.Printf("Purged %d content items\n", result.DeletedCount)
	}

	// Get template IDs
	templates := make(map[string]primitive.ObjectID)
	cursor, _ := db.Collection("templates").Find(ctx, bson.M{})
	var tmpls []Template
	cursor.All(ctx, &tmpls)
	for _, t := range tmpls {
		templates[t.Slug] = t.ID
		fmt.Printf("Template: %s -> %s\n", t.Slug, t.ID.Hex())
	}

	now := time.Now()

	// Define all pages to migrate with EXACT text from metavert.io
	pages := []PageContent{
		// Homepage - EXACT TEXT from metavert.io (extracted via curl)
		{
			Title:           "Metavert",
			Slug:            "",
			TemplateSlug:    "homepage",
			MetaDescription: "Metavert is a venture studio and management consultancy that advises, supports and analyzes the businesses creating a future based on the metaverse.",
			Data: map[string]interface{}{
				"hero_tagline": `<p class="intro-types">extrovert | introvert | <strong><em>metavert</em></strong></p>
<h3><strong>metavert, n.</strong>: a person who is most expressive in the metaverse.</h3>
<h3><strong>metaverse, n.</strong>: the <em>next generation of the internet</em>; oriented around real-time, immersive activity; powered by an exponential rise in creators.</h3>`,
				"intro_content": `<h2>Building the Metaverse</h2>
<p>Metavert is a venture studio and management consultancy that advises, supports and analyzes the businesses creating a future based on the metaverse.</p>
<h4>Contact the Metavert</h4>
<p><a href="mailto:jon@metavert.io">jon {at} metavert.io</a> | <a href="https://linkedin.com/in/jonradoff" target="_blank">LinkedIn</a> | <a href="https://twitter.com/jradoff" target="_blank">Twitter</a></p>`,
				"sections": `<h2>Areas of Interest</h2>
<div class="areas-of-interest">
	<div class="area-card">
		<h3>Creator Economy</h3>
		<p>The World Wide Web transformed human communication by digitizing publishing and commerce. The next era will exponentially expand the frontiers of creativity within dematerialized spaces: <a href="/virtual-world">virtual worlds</a> and virtual societies, along with the <a href="/low-code-platform">low-code tools</a> that make it easy to contribute.</p>
	</div>
	<div class="area-card">
		<h3>Decentralization</h3>
		<p>The internet was created as an open ecosystem: the domain name system and the Web are inherently <a href="/decentralization">decentralized</a>—yet in recent history, powerful platforms have gained disproportionate power. The next generation will (re)decentralize via open source, interoperability, <a href="/blockchain">blockchain</a>, <a href="/smart-contract">smart contracts</a> and self-sovereignty.</p>
	</div>
	<div class="area-card">
		<h3>Real-Time Technology</h3>
		<p>The metaverse is real-time: <a href="/games">games</a> and <a href="/immersive-social">social experiences</a> that are responsive and immersive, supported by spatial computing technologies such as <a href="/3d-engine">3D rendering</a>, <a href="/ray-tracing">ray tracing</a> and <a href="/virtual-reality">virtual reality</a>; <a href="/machine-intelligence">artificial intelligence</a>, deep learning and virtual beings; new, more ergonomic human interfaces; and the edge computing <a href="/infrastructure">infrastructure</a> and <a href="/distributed-network">high-speed networking</a> to support it.</p>
	</div>
	<div class="area-card">
		<h3>Games</h3>
		<p>Games have always been a key driver of new computer technology. The <a href="/gametech">GameTech</a> stack—<a href="/3d-engine">3D rendering</a>, <a href="/virtual-world">virtual worlds</a> and <a href="/live-services">live services</a>—will power the metaverse, just as trends like multiplayer worlds, <a href="/esports">esports</a>, play-to-earn/create-to-earn and modding will define the next decades of culture.</p>
	</div>
	<div class="area-card">
		<h3>Ubiquitous Computing</h3>
		<p>The metaverse is not a place we'll simply visit—it will surround us as we <a href="/spatial-computing">embed computing into all aspects of the physical world</a>. We will create new means of accessing it such as <a href="/augmented-reality">augmented reality,</a> smart glasses and brain-computer interfaces; and we will feed information into the metaverse via billions of people and trillions of interconnected devices, sensors and programs.</p>
	</div>
	<div class="area-card">
		<h3>Machine Intelligence</h3>
		<p>We are at the dawn of <a href="/machine-intelligence">human-machine collaboration</a>. The metaverse will be be populated by virtual beings who will assist us, support our creativity and even befriend us. A range of new artificial intelligence algorithms will enable us to curate and optimize our experience. Computers will learn to relate to us on our terms—through voice, gesture and even our minds—rather than through code.</p>
	</div>
</div>
<h3>Read More</h3>
<p>The <a href="https://medium.com/building-the-metaverse" target="_blank">Building the Metaverse blog</a> is where you can learn more about what to make of the coming decade of the internet. A few good starting points:</p>
<ul>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">The Metaverse Market Map</a> — showing the organization of the industry, including 200+ companies across 7 layers.</li>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a> — covering some of the huge social and technological shifts that are driving change.</li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a> — categorizing the metaverse industry into 7 layers: experiences, discovery, creator economy, spatial computing, decentralization, human interface, and infrastructure.</li>
</ul>`,
				"quote_text":   "Many people think of the metaverse as 3D space that will surround us. But the metaverse is not 3D or 2D, or even necessarily graphical; it is about the inexorable dematerialization of physical space, distance, and objects.",
				"quote_author": "Jon Radoff, The Metaverse Value Chain",
			},
		},

		// Projects page
		{
			Title:           "Projects",
			Slug:            "projects",
			TemplateSlug:    "standard-page",
			MetaDescription: "Metavert portfolio companies and advisory relationships building the future of the metaverse.",
			Data: map[string]interface{}{
				"content": `<div class="feature-grid">
	<div class="feature-card">
		<h3>Beamable</h3>
		<p><strong>Role:</strong> Co-Founder</p>
		<p>The technology stack that will power the metaverse was born in games—what is called <em>GameTech.</em> The main components are the 3D engine that provides visualization of immersive environments—and the online infrastructure (worlds, social connection, persistence) that gives experiences meaning and life beyond the screen.</p>
		<p>Beamable is focused on this live game infrastructure: games today, and the experiences of the metaverse tomorrow.</p>
		<p><a href="https://www.gamesindustry.biz/articles/2021-11-01-beamable-raises-over-usd10m-in-funding" target="_blank">Read about Beamable's $10M+ in Funding Announcement →</a></p>
	</div>
	<div class="feature-card">
		<h3>Metamundo</h3>
		<p><strong>Role:</strong> Advisor</p>
		<p>Building the world's first 3D NFT marketplace for building the metaverse, supporting creators with the ability to deploy assets across Decentraland, Somnium, Cryptovoxels and other 3D worlds.</p>
		<p><a href="https://finance.yahoo.com/news/metamundo-raises-2-7m-launch-150000244.html" target="_blank">Read the Metamundo funding announcement →</a></p>
	</div>
</div>`,
			},
		},

		// Contact page
		{
			Title:           "Contact",
			Slug:            "contact",
			TemplateSlug:    "standard-page",
			MetaDescription: "Get in touch with Metavert. Let me know how I can help.",
			Data: map[string]interface{}{
				"content": `<h2>Input/Output</h2>
<p>Let me know how I can help!</p>

<div class="contact-form" id="contact-form">
	<div id="form-message" class="form-message" style="display:none;"></div>
	<form id="contact-form-element">
		<div class="form-group">
			<label for="name">Name *</label>
			<input type="text" id="name" name="name" required>
		</div>
		<div class="form-group">
			<label for="email">Email *</label>
			<input type="email" id="email" name="email" required>
		</div>
		<div class="form-group">
			<label for="message">Message *</label>
			<textarea id="message" name="message" required></textarea>
		</div>
		<button type="submit">Send</button>
	</form>
</div>

<script>
document.getElementById('contact-form-element').addEventListener('submit', function(e) {
	e.preventDefault();
	var form = this;
	var messageDiv = document.getElementById('form-message');
	var submitBtn = form.querySelector('button[type="submit"]');

	submitBtn.disabled = true;
	submitBtn.textContent = 'Sending...';

	var formData = new FormData(form);

	fetch('/api/contact', {
		method: 'POST',
		body: new URLSearchParams(formData)
	})
	.then(function(res) { return res.json(); })
	.then(function(data) {
		messageDiv.style.display = 'block';
		if (data.success) {
			messageDiv.className = 'form-message success';
			messageDiv.textContent = 'Thank you!';
			form.reset();
		} else {
			messageDiv.className = 'form-message error';
			messageDiv.textContent = data.error;
		}
		submitBtn.disabled = false;
		submitBtn.textContent = 'Send';
	})
	.catch(function(err) {
		messageDiv.style.display = 'block';
		messageDiv.className = 'form-message error';
		messageDiv.textContent = 'An error occurred. Please try again.';
		submitBtn.disabled = false;
		submitBtn.textContent = 'Send';
	});
});
</script>`,
			},
		},

		// Metavert.TV page
		{
			Title:           "Metavert.TV",
			Slug:            "metaverttv",
			TemplateSlug:    "standard-page",
			MetaDescription: "Metavert.TV is a live-streamed program featuring CEOs, CTOs and other leaders from the game industry, decentralized technology and artificial intelligence industries.",
			Data: map[string]interface{}{
				"content": `<p>Metavert.TV is a live-streamed program featuring CEOs, CTOs and other leaders from the game industry, decentralized technology and artificial intelligence industries.</p>

<p>The platform broadcasts simultaneously across X, YouTube, LinkedIn, TikTok, and Facebook, typically engaging over 1,000 live audience members with approximately 200,000 replays per episode. Programming emphasizes conversational, free-form discussions with live audience participation. The viewership comprises entrepreneurs, investors, and enthusiasts focused on blockchain, AI, videogames, and metaverse topics.</p>

<h2>Watch</h2>
<ul>
	<li><a href="https://youtube.com/@metavert" target="_blank">YouTube Channel</a></li>
	<li><a href="https://twitter.com/jradoff" target="_blank">X (Twitter)</a></li>
	<li><a href="https://linkedin.com/in/jonradoff" target="_blank">LinkedIn</a></li>
	<li><a href="https://tiktok.com/@metavert" target="_blank">TikTok</a></li>
</ul>`,
			},
		},

		// Concepts Glossary index
		{
			Title:           "Concepts & Glossary",
			Slug:            "concepts-glossary",
			TemplateSlug:    "standard-page",
			MetaDescription: "A comprehensive glossary of metaverse concepts, technologies, and terms.",
			Data: map[string]interface{}{
				"content": `<p>Explore the key concepts and technologies shaping the metaverse.</p>

<h2>By Market Layer</h2>
<ul>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/discovery">Discovery</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/human-interface">Human Interface</a></li>
	<li><a href="/infrastructure">Infrastructure</a></li>
</ul>

<h2>Technologies</h2>
<ul>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/distributed-network">Distributed Network</a></li>
	<li><a href="/gametech">GameTech</a></li>
	<li><a href="/live-services">Live Services</a></li>
	<li><a href="/low-code-platform">Low Code Platform</a></li>
	<li><a href="/ray-tracing">Ray Tracing</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
</ul>

<h2>Experiences & Content</h2>
<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/esports">Esports</a></li>
	<li><a href="/immersive-social">Immersive Social</a></li>
	<li><a href="/virtual-world">Virtual World</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token (NFT)</a></li>
</ul>

<h2>Megatrends</h2>
<ul>
	<li><a href="/megatrends">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="/market-layer">Market Layer Framework</a></li>
</ul>`,
			},
		},

		// Privacy Policy
		{
			Title:           "Privacy Policy",
			Slug:            "privacy-policy",
			TemplateSlug:    "standard-page",
			MetaDescription: "Metavert privacy policy describing how we collect, use, and protect your personal information.",
			Data: map[string]interface{}{
				"content": `<p><em>Last updated: October 31, 2021</em></p>

<h2>Information Collection</h2>
<p>We collect personal information that you voluntarily provide to us when you express an interest in obtaining information about us.</p>
<p>Categories include email addresses, names, contact data, IP addresses, browser characteristics, and device information collected automatically during website visits.</p>

<h2>Data Usage</h2>
<p>We process information for "administrative information," "protecting our Services," marketing communications, and "deliver targeted advertising to you."</p>

<h2>Data Sharing</h2>
<p>Information may be shared with data analytics services, ad networks, and website hosting providers. We do not share, sell, rent or trade any of your information with third parties for their promotional purposes.</p>

<h2>Retention</h2>
<p>We retain personal data only as long as necessary with a general maximum of 90 days unless legal requirements dictate otherwise.</p>

<h2>User Rights</h2>
<p>Residents of California and the European Economic Area have specific rights including access requests, deletion, and data portability. Users can submit requests via <a href="mailto:privacy@metavert.io">privacy@metavert.io</a>.</p>

<h2>Cookies & Tracking</h2>
<p>We permit cookies and tracking technologies, with opt-out available through standard browser controls.</p>

<h2>Contact</h2>
<p>For privacy inquiries, contact: <a href="mailto:info@metavert.io">info@metavert.io</a></p>`,
			},
		},

		// Terms of Service
		{
			Title:           "Terms of Service",
			Slug:            "terms-of-service",
			TemplateSlug:    "standard-page",
			MetaDescription: "Terms of service governing the use of metavert.io website and services.",
			Data: map[string]interface{}{
				"content": `<p><em>Last updated: July 3, 2024</em></p>

<h2>1. Application of Terms</h2>
<p>Terms apply to use of the Metavert website, a service of Metavert LLC organized in Wyoming. By accessing the site, you agree to these terms. If acting on behalf of another person or entity, you confirm authorization to bind that party.</p>
<p>Non-agreement means you cannot access the site and must stop immediately.</p>

<h2>2. Changes</h2>
<p>We may change these Terms at any time by updating them on the Website. Unless stated otherwise, any change takes effect immediately.</p>
<p>Users remain responsible for staying current. Continued use implies acceptance of modified terms. The company may change, suspend, discontinue, or restrict website access without notice.</p>

<h2>3. Your Obligations</h2>
<p>Users must provide true, current, complete information and update it promptly. If you are given a User ID, you must keep your User ID secure.</p>
<p>Users cannot disclose User IDs or permit others to use them. Unauthorized access requires immediate notification to <a href="mailto:info@metavert.io">info@metavert.io</a>.</p>
<p>Users must not compromise the website or underlying systems, use viruses, malware, or interference methods, or access via non-standard methods (scraping, bots, data mining).</p>
<p>Users must defend, indemnify, and hold us harmless from third-party claims arising from website use, term breaches, or rights violations.</p>

<h2>4. Intellectual Property</h2>
<p>The company and licensors own all proprietary and intellectual property rights in the website, including text, graphics, trademarks, and content.</p>
<p>"Metavert" is a registered trademark in the United States; the logo is pending trademark status.</p>
<p>Creative Commons Attribution licensed content may be used if users indicate Metavert as the source and link to original material. Logos, trademarks, and unlicensed pages are excluded.</p>

<h2>5. Disclaimers</h2>
<p>To the extent permitted by law, we and our licensors have no liability or responsibility to you or any other person for any Loss.</p>
<p>This covers website unavailability, errors in information, virus exposure, and linked sites.</p>
<p>All Content here is information of a general nature and does not address the circumstances of any particular individual or entity. It doesn't constitute professional or financial advice. Users assume sole responsibility for evaluating information before decisions.</p>

<h2>6. Liability</h2>
<p>To the maximum extent permitted by law: you access and use the Website at your own risk; and we are not liable or responsible to you or any other person for any Loss.</p>
<p>Where applicable, liability caps at amounts users actually paid to the company.</p>
<p>DMCA notifications reach: <a href="mailto:copyright@metavert.io">copyright@metavert.io</a> or Jonathan Radoff, Metavert LLC, 30 N Gould St Ste 21426, Sheridan, WY 82801.</p>

<h2>7. Governing Law</h2>
<p>Disputes are governed by Massachusetts law. Parties must attempt negotiation/mediation under American Arbitration Association rules for 90 days before arbitration.</p>
<p>If for any matter that requires a court proceeding, each party submits to the exclusive jurisdiction of the Courts of Massachusetts.</p>
<p>The company may seek injunctive relief for intellectual property violations without mediation requirements.</p>

<h2>8. Contact</h2>
<p>For questions about these terms, contact: <a href="mailto:info@metavert.io">info@metavert.io</a></p>`,
			},
		},
	}

	// Concept pages with EXACT text from metavert.io
	conceptPages := []PageContent{
		{
			Title:           "Virtual World",
			Slug:            "virtual-world",
			TemplateSlug:    "concept-page",
			MetaDescription: "Virtual Worlds are persistent environments that enable interaction and persistence between multiple people.",
			Data: map[string]interface{}{
				"definition": `<p>Virtual Worlds are persistent environments that enable interaction and persistence between multiple people.</p>

<p>This includes non-graphical environments dating back to multiuser dungeons (MUDs) and bulletin board games—and since around the late 1990's, more graphical massively multiplayer online games (MMOs and MMORPGs).</p>

<p>Virtual Worlds frequently have virtual economies with currencies and <a href="/virtual-item"><strong>virtual items</strong></a> as well as opportunities for <a href="/creator-economy"><strong>creators</strong></a> to contribute and customize.</p>`,
				"topic_links": `<ul>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/gametech">GameTech</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-experiences-of-the-metaverse-2126a7899020" target="_blank">Experiences of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://www.gamedeveloper.com/business/types-of-game-currencies-in-mobile-free-to-play" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://www.playableworlds.com/news/riffs-by-raph:-how-virtual-worlds-work-part-1/" target="_blank">How Virtual Worlds Work</a>, series by Raph Koster, leading innovators of MMOs.</li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a>, compare/contrast of closed centralized economies (Fortnite), open centralized (Minecraft) and open decentralized (Decentraland).</li>
</ul>`,
			},
		},
		{
			Title:           "Blockchain",
			Slug:            "blockchain",
			TemplateSlug:    "concept-page",
			MetaDescription: "A blockchain is a distributed ledger that uses cryptography to validate transactions.",
			Data: map[string]interface{}{
				"definition": `<p><a href="/megatrends"><strong>Megatrend</strong></a></p>

<p>A blockchain is a distributed ledger that uses cryptography to validate transactions. The advantage of blockchain is that financial transactions can be recorded in a <strong>decentralized</strong>, <strong>trustless</strong> and <strong>permissionless</strong> manner, without requiring any central authority that controls it.</p>

<p>The first major blockchain application was the cryptocurrency Bitcoin, invented by pseudonymous creator Satoshi Nakamoto in 2009.</p>

<p>In 2015, the Ethereum blockchain introduced the concept of <a href="/smart-contract"><strong>smart contracts</strong></a>, which allows programmatic exchanges of value: for the first time, code could exchange value (currencies, assets) with other code (much as the World Wide Web allows code to exchange information). This has enabled new forms of decentralized software to be created including <a href="/decentralized-finance"><strong>decentralized finance (DeFi)</strong></a>, <a href="/decentralized-autonomous-organization"><strong>decentralized autonomous organizations (DAOs)</strong></a> and <a href="/non-fungible-token"><strong>non fungible tokens (NFTs)</strong></a>.</p>

<p>The first blockchains including Bitcoin and Ethereum use proof-of-work algorithms that require substantial energy and computing power to complete the cryptographic algorithms that allow for secure transactions. <a href="/proof-of-stake"><strong>Proof-of-stake</strong></a> blockchains (including Ethereum 2.0 and other next-generation blockchains) require comparatively trivial amounts of computation or energy.</p>`,
				"topic_links": `<ul>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="http://unenumerated.blogspot.com/2017/02/money-blockchains-and-social-scalability.html" target="_blank">Money, Blockchains and Social Scalability</a> (Nick Szabo)</li>
	<li><a href="https://www.bitdegree.org/crypto/tutorials/proof-of-work-vs-proof-of-stake" target="_blank">Proof-of-Work vs. Proof-of-Stake</a></li>
	<li><a href="https://101blockchains.com/smart-contracts/" target="_blank">Smart Contracts: the Ultimate Guide for Beginners</a> (Hasib Anwar)</li>
	<li><a href="https://medium.com/@rkmjcharpb/a-defi-stream-of-consciousness-and-the-opportunity-for-trustless-games-a4f3b34cf0f0" target="_blank">A DeFi Stream of Consciousness and the opportunity for "trustless games"</a> (Richard Kim)</li>
	<li><a href="https://www.youtube.com/watch?v=SYPzqRaN4zA" target="_blank">Decentralized Autonomous Organizations (DAOs)</a>, from Stanford</li>
</ul>`,
			},
		},
		{
			Title:           "Artificial Intelligence",
			Slug:            "artificial-intelligence",
			TemplateSlug:    "concept-page",
			MetaDescription: "Artificial intelligence (AI or machine intelligence) is the domain of computer science that involves programming computers to operate in ways we perceive as intelligent.",
			Data: map[string]interface{}{
				"definition": `<p><a href="/megatrends"><strong>Megatrend</strong></a></p>

<p>Artificial intelligence (also referred to as "AI" or "machine intelligence") is the domain of computer science that involves programming computers to operate in ways we perceive as intelligent.</p>

<p>Is the intelligence truly "artificial" or is it simply a new form of intelligence?</p>

<p>For many years, although chess programs could be taught to play—few people believed a computer could beat a chess grandmaster—until Gary Kasparov was defeated by Deep Blue. And then few believed a computer could beat a master at Go, a far more complex game—until AlphaGo did so.</p>

<p>Today, machines are using deep learning to acquire more of the capabilities that were once exclusively human: autonomous driving, recognizing images, and interpreting natural language (such as with GPT-3).</p>

<p>Machines are even beginning to supplement human creativity, by providing automatically-generated code, or generating scenes and character animations. This is a field referred to as generative AI.</p>

<p>In the metaverse, machine intelligence supports advanced interfaces that interpret our evolved expressions through speech, gesture, and even emotion; they'll be used to simulate various aspects of the physical world; and they'll play the role of characters and virtual beings.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/low-code-platform">Low Code Platform</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-and-artificial-intelligence-1f26f1d37e90" target="_blank">The Metaverse and Artificial Intelligence</a></li>
</ul>`,
			},
		},
		{
			Title:           "Decentralization",
			Slug:            "decentralization",
			TemplateSlug:    "concept-page",
			MetaDescription: "Decentralization is the set of technologies, design patterns and practices that shift power and control away from centralized authorities.",
			Data: map[string]interface{}{
				"definition": `<p>Decentralization is the set of technologies, design patterns and practices that shift power and control away from centralized authorities (such as walled gardens and financial institutions).</p>

<p>The internet was originally designed as a highly decentralized network. For some technologies, such as the domain name system (DNS) or the World Wide Web—it still is. But over time, the need for simplicity and access to audiences eventually favored a number of powerful, centralized platforms. However, as new technologies built on open source, open standards and blockchain emerge, this power dynamic may shift back towards individual creators and projects, potentially increasing disruptive innovation.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-permissionless-metaverse-f0c01ed3e9f6" target="_blank">The Permissionless Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/so-you-want-to-compete-with-roblox-d83b7f5ecf43" target="_blank">So You Want to Compete with Roblox</a></li>
	<li><a href="https://cdixon.org/2018/02/18/why-decentralization-matters" target="_blank">Why Decentralization Matters</a></li>
	<li><a href="https://marginalrevolution.com/marginalrevolution/2021/06/decentralization-counter-decentralization-and-re-decentralization.html" target="_blank">Decentralization, Counter-Decentralization, and Redecentralization</a></li>
</ul>`,
			},
		},
		{
			Title:           "GameTech",
			Slug:            "gametech",
			TemplateSlug:    "concept-page",
			MetaDescription: "Game Technology (or 'GameTech') is the set of technologies that have made modern video gaming possible.",
			Data: map[string]interface{}{
				"definition": `<p>Game Technology (or "GameTech") is the set of technologies that have made modern video gaming possible.</p>

<p>It is the GameTech stack that will largely enable the experiences of the metaverse, and game-oriented designs informed by emotion and storytelling that will shape the way these experiences are structured.</p>

<p>GameTech software includes 3D engines to enable the rendering of immersive graphics, as well as the live services infrastructure that enables persistent virtual worlds and games with sophisticated communities and economies. GameTech hardware includes the Graphics Processing Unit (GPU).</p>`,
				"topic_links": `<ul>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/live-services">Live Services</a></li>
	<li><a href="/virtual-world">Virtual World</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-is-real-gamification-c2b74b9dce05" target="_blank">The Metaverse is Real Gamification</a></li>
</ul>`,
			},
		},
		{
			Title:           "3D Engine",
			Slug:            "3d-engine",
			TemplateSlug:    "concept-page",
			MetaDescription: "3D Engines are software that enable real-time generation of 3-dimensional graphics.",
			Data: map[string]interface{}{
				"definition": `<p>3D Engines are software that enable real-time generation of 3-dimensional graphics (in contrast to pre-rendered animation that appears in movies). Real-time 3D makes it possible to interact with a game, virtual world or other graphically immersive application.</p>

<p>This software is an important subset of the spatial computing domain; it maps the geometric representation of objects in 3D space into the methods used by a GPU (Graphics Processing Unit) to deliver the visual output that can be displayed on screens.</p>

<p>3D engines are one of the fundamental elements of the GameTech stack.</p>

<p>The leading vendors of 3D Engine technology include Unity and Epic Games (Unreal Engine).</p>`,
				"topic_links": `<ul>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/gametech">GameTech</a></li>
	<li><a href="/ray-tracing">Ray Tracing</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">Market Map of the Metaverse</a></li>
	<li><a href="https://www.techspot.com/article/650-history-of-the-gpu/" target="_blank">History of the Modern Graphics Processor</a></li>
	<li><a href="https://www.youtube.com/watch?v=ELkhD7FaBMU" target="_blank">How Games Engines Work!</a> by TheHappieCat</li>
	<li><a href="https://www.youtube.com/watch?v=JKHqY4qrzKE" target="_blank">The Physics of Light and Rendering</a> (John Carmack)</li>
</ul>`,
			},
		},
		{
			Title:           "Spatial Computing",
			Slug:            "spatial-computing",
			TemplateSlug:    "concept-page",
			MetaDescription: "Spatial computing is the technology that immerses humans into the computing environment, and adds computing to objects in the spatial environment around us.",
			Data: map[string]interface{}{
				"definition": `<p>Spatial computing is the technology that immerses humans into the computing environment, and adds computing to objects in the spatial environment around us.</p>

<p>This includes technology for generating output (such as 3D graphics or spatial audio); technology like image recognition and gesture recognition for facilitating interacting in these environments; advanced user interfaces for synthesizing the data from digital twins; and geospatial information to merge local-scale information with the rest of the world.</p>

<p>The software within spatial computing enables a large range of human interface hardware including traditional screens, as well as augmented reality, virtual reality and extended reality.</p>`,
				"topic_links": `<ul>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://acg.media.mit.edu/people/simong/thesis/SpatialComputing.pdf" target="_blank">Spatial Computing</a> by Simon Greenwold—the original paper on this topic</li>
	<li><a href="https://www.mdpi.com/2220-9964/9/7/439" target="_blank">Geospatial Information Visualization and Extended Reality Displays</a></li>
</ul>`,
			},
		},
		{
			Title:           "Virtual Reality",
			Slug:            "virtual-reality",
			TemplateSlug:    "concept-page",
			MetaDescription: "Virtual Reality (VR) is technology that allows you to be fully immersed into a virtual world.",
			Data: map[string]interface{}{
				"definition": `<p>Virtual Reality (VR) is technology that allows you to be fully immersed into a virtual world. Screens are attached directly to one's head, so that head and eye-tracking makes it possible to look around at the virtual environment. This is in contrast to Augmented Reality, which layers the spatial computing environment on top of the physical world around you.</p>`,
				"topic_links": `<ul>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/human-interface">Human Interface</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
</ul>`,
			},
		},
		{
			Title:           "Augmented Reality",
			Slug:            "augmented-reality",
			TemplateSlug:    "concept-page",
			MetaDescription: "Augmented Reality (AR) is technology that allows you to add information to the visual environment around you.",
			Data: map[string]interface{}{
				"definition": `<p>Augmented Reality (AR) is technology that allows you to be add information to the visual environment around you. Examples would include recognizing and adding information to objects or generating digital holograms in physical space. This is in contrast to virtual reality which is a completely immersed experience.</p>`,
				"topic_links": `<ul>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/human-interface">Human Interface</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
</ul>`,
			},
		},
		{
			Title:           "Games",
			Slug:            "games",
			TemplateSlug:    "concept-page",
			MetaDescription: "Games are one of the oldest social technologies known to humankind.",
			Data: map[string]interface{}{
				"definition": `<p>Games are one of the oldest social technologies known to humankind: board games and gaming tokens have been discovered that are over 5,000 years old.</p>

<p>For as long as there have been games on computers, they've moved technology forward in innovative and sometimes unexpected ways. Games are one of the drivers between real-time networking as well as graphics processing units. These enabling hardware and software technologies—GameTech—will be what the metaverse is built upon.</p>

<p>Games have given rise to several adjacent categories of experience including immersive social and esports.</p>`,
				"topic_links": `<ul>
	<li><a href="/gametech">GameTech</a></li>
	<li><a href="/immersive-social">Immersive Social</a></li>
	<li><a href="/esports">Esports</a></li>
	<li><a href="/experiences">Experiences</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.washingtonpost.com/news/speaking-of-science/wp/2018/02/13/archaeologists-puzzled-over-immaculate-5000-year-old-board-game-pieces/" target="_blank">Archaeologists puzzled over immaculate, 5,000-year-old board game pieces</a></li>
	<li><a href="https://medium.com/building-the-metaverse/experiences-of-the-metaverse-2e8e09e1bbf7" target="_blank">Experiences of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/game-economics-part-1-attention-3f0958a7c06f" target="_blank">Game Economics, Part 1: The Attention Economy</a></li>
	<li><a href="https://medium.com/building-the-metaverse/game-development-trends-in-2021-67b3ac16ed46" target="_blank">Game Development Trends in 2021</a>: solo-to-social; technologists-to-artists; games-to-economies</li>
	<li><a href="https://medium.com/building-the-metaverse/introduction-to-game-thinking-d67b7c53b2ef" target="_blank">Introduction to Game Thinking</a></li>
	<li><a href="https://nicolelazzaro.com/the4-keys-to-fun/" target="_blank">Games and the Four Keys to Fun</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-is-real-gamification-c2b74b9dce05" target="_blank">The Metaverse is Real Gamification</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-game-currencies-in-mobile-free-to-play-e552cadbda91" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://quanticfoundry.com/2019/04/03/a-closer-look-into-the-12-gamer-motivations-from-the-gamer-motivation-model/" target="_blank">A Closer Look into the 12 Gamer Motivations</a></li>
	<li><a href="https://www.raphkoster.com/2018/03/16/how-virtual-worlds-work/" target="_blank">How Virtual Worlds Work</a></li>
</ul>`,
			},
		},
		{
			Title:           "Smart Contract",
			Slug:            "smart-contract",
			TemplateSlug:    "concept-page",
			MetaDescription: "A smart contract is a means of programmatically exchanging value on the Internet using blockchain.",
			Data: map[string]interface{}{
				"definition": `<p>A smart contract is a means of programmatically exchanging value (currency and assets) on the Internet using blockchain.</p>

<p>Prior to smart contracts, value exchange occurred through:</p>
<ul>
	<li>Meeting in person and exchanging actual cash (e.g., trading dollars in your pocket for the item you want to purchase)</li>
	<li>Using a trusted, centralized authority. For example, a bank, brokerage or escrow company would be trusted to finalize a transaction for you.</li>
</ul>

<p>In contrast, smart contracts allow you to write code that exchanges value without any parties needing to trust each other, meet each other and without the intervention of third-party authorities.</p>

<p>Smart contracts have the potential to be as disruptive to finance, art, collecting, real estate, gaming and other industries in the same way that decentralized Web publishing disrupted many other information-intensive industries.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://101blockchains.com/smart-contracts/" target="_blank">Smart Contracts: The Ultimate Guide for Beginners</a></li>
</ul>`,
			},
		},
		{
			Title:           "Non-Fungible Token",
			Slug:            "non-fungible-token",
			TemplateSlug:    "concept-page",
			MetaDescription: "A non-fungible token (NFT) is a way to assign ownership to asset on the blockchain.",
			Data: map[string]interface{}{
				"definition": `<p>A non-fungible token (NFT) is a way to assign ownership to asset on the blockchain. It is implemented using a smart contract; on Ethereum the standard is defined by ERC-721 and ERC-1155.</p>

<p>An easy way to think about the distinction between NFTs and a cryptocurrency is that currency is fungible, i.e., you don't care about one unit or another. For example, if you have a dollar bill you're normally willing to exchange it for any other, regardless of the serial number on the bill; this is what makes it fungible. A non-fungible asset would be a house you live in—it has unique properties that make it distinct from all others. Similarly, the title system for real estate is a non-digital means of tracking ownership that is a helpful metaphor for NFTs.</p>

<p>Unlike virtual items that existed before NFTs, the blockchain enables several capabilities that didn't exist previously: permanence; decentralization; trustless programmability; provable provenance; provable scarcity. The result is a new class of disruptive applications, artwork and gaming—as well as new business models including play-to-earn and others built around decentralized marketplaces.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/game-economics-part-2-nfts-and-digital-collectibles-3ec3e456f1df" target="_blank">Game Economics, Part 2: NFTs and Digital Collectibles</a></li>
	<li><a href="https://twitter.com/jonlai/status/1417873626680299522" target="_blank">Why do play-to-earn games work when real money trading in games has (mostly) not worked?</a> (Jon Lai)</li>
	<li><a href="https://metaversal.banklesshq.com/p/how-to-approach-the-generative-art" target="_blank">How to Approach the Generative Art Boom</a> (William Peaster)</li>
	<li><a href="https://pierskicks.medium.com/into-the-void-where-crypto-meets-the-metaverse-f44f2f09ffa2" target="_blank">Into the Void: where Crypto Meets the Metaverse</a> (Piers Kicks)</li>
	<li><a href="https://cdixon.org/2021/02/27/nfts-and-a-thousand-true-fans" target="_blank">NFTs and a Thousand True Fans</a> (Chris Dixon)</li>
	<li><a href="https://www.theverge.com/22310188/nft-explainer-what-is-blockchain-crypto-art-faq" target="_blank">What the Hell is an NFT?</a></li>
</ul>`,
			},
		},
		{
			Title:           "Live Services",
			Slug:            "live-services",
			TemplateSlug:    "concept-page",
			MetaDescription: "Live Services are the internet-based software platforms that enable virtual worlds and online games with sophisticated economies and communities.",
			Data: map[string]interface{}{
				"definition": `<p>Live Services are the internet-based software platforms—typically cloud-based—that enable virtual worlds and online games with sophisticated economies and communities. This includes software for managing the inventory of virtual economies, social features, online events and tournaments, regular updates and other features that are necessary to the operation of games and other metaverse experiences.</p>`,
				"topic_links": `<ul>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/gametech">GameTech</a></li>
	<li><a href="/virtual-world">Virtual World</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">Market Map of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-game-currencies-in-mobile-free-to-play-e552cadbda91" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a>, compare/contrast of closed centralized economies (Fortnite), open centralized (Minecraft) and open decentralized (Decentraland)</li>
</ul>`,
			},
		},
		{
			Title:           "Low Code Platform",
			Slug:            "low-code-platform",
			TemplateSlug:    "concept-page",
			MetaDescription: "Low code platforms allow creators to craft applications with little or no computer coding.",
			Data: map[string]interface{}{
				"definition": `<p>Low code platforms allow creators to craft applications with little (or in the case of a no-code platforms, no) computer coding.</p>

<p>Low code and no-code platforms result in an exponential increase in the number of people who are able to create applications and experiences.</p>

<p>Today, many systems built around generative AI allow users to enter "text prompts" that implement code, or implement application behaviors without knowledge of computer languages.</p>`,
				"topic_links": `<ul>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/megatrends">Megatrends</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
</ul>`,
			},
		},
		{
			Title:           "Ray Tracing",
			Slug:            "ray-tracing",
			TemplateSlug:    "concept-page",
			MetaDescription: "Ray Tracing refers to rendering graphics by simulating the physics of light and materials.",
			Data: map[string]interface{}{
				"definition": `<p>Most real-time rendering that happens in games as of 2021 are still performed using a collection of computational and mathematical tricks.</p>

<p>In contrast, <strong>Ray Tracing</strong> refers to rendering graphics by simulating the physics of light and materials. As <strong>graphics processing units</strong> and <strong>3D engines</strong> improve, it may become commonplace to use this technology in games and metaverse <strong>experiences</strong>.</p>`,
				"topic_links": `<ul>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/gametech">GameTech</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.youtube.com/watch?v=JKHqY4qrzKE" target="_blank">The Physics of Light and Rendering</a>, talk by John Carmack</li>
</ul>`,
			},
		},
		{
			Title:           "Distributed Network",
			Slug:            "distributed-network",
			TemplateSlug:    "concept-page",
			MetaDescription: "The internet was designed as a distributed, decentralized network intended to survive natural disasters and nuclear strikes.",
			Data: map[string]interface{}{
				"definition": `<p>The internet was designed as a distributed, <strong>decentralized</strong> network intended to survive natural disasters and nuclear strikes.</p>

<p>Over time, many applications came to depend on cloud computing—servers running at remote facilities—to deliver their capabilities. Think of cloud computing as "computing on tap," not unlike a water or electric utility.</p>

<p>Today, the speed of the internet is growing both faster (due to <strong>5G</strong> and <strong>6G</strong>) as well as more distributed than the current cloud computing infrastructure; <strong>edge computing</strong> is moving more of the cloud-based infrastructure closer to the end user, to allow for much faster coordination needed by applications such as gaming and artificial intelligence.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/infrastructure">Infrastructure</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
</ul>`,
			},
		},
		{
			Title:           "Immersive Social",
			Slug:            "immersive-social",
			TemplateSlug:    "concept-page",
			MetaDescription: "Immersive Social is a category of experiences in the metaverse that are geared towards social interaction.",
			Data: map[string]interface{}{
				"definition": `<p>Immersive Social is a category of experiences in the metaverse that are geared towards social interaction; they may be thought of as an embodied, real-time evolution of social networks and chat.</p>

<p>Examples include <strong>VRchat</strong>, <strong>Rec Room</strong> and many of the experiences people enjoy in <strong>Roblox</strong>.</p>`,
				"topic_links": `<ul>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/virtual-world">Virtual World</a></li>
	<li><a href="/games">Games</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/experiences-of-the-metaverse-2e8e09e1bbf7" target="_blank">Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		{
			Title:           "Infrastructure",
			Slug:            "infrastructure",
			TemplateSlug:    "concept-page",
			MetaDescription: "Infrastructure is the set of fundamental technologies that the rest of the metaverse is built upon.",
			Data: map[string]interface{}{
				"definition": `<p>Infrastructure is the set of fundamental technologies that the rest of the metaverse is built upon.</p>

<p>This encompasses semiconductors (particularly Graphics Processing Units), networks, cloud-based services, data centers, edge computing, batteries and material science.</p>`,
				"topic_links": `<ul>
	<li><a href="/market-layer">Market Layer</a></li>
	<li><a href="/distributed-network">Distributed Network</a></li>
	<li><a href="/gametech">GameTech</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">Market Map of the Metaverse</a></li>
</ul>`,
			},
		},
		{
			Title:           "Creator Economy",
			Slug:            "creator-economy",
			TemplateSlug:    "concept-page",
			MetaDescription: "The creator economy is the combination of software and marketplaces that make it possible for creative people and teams to add content to the metaverse.",
			Data: map[string]interface{}{
				"definition": `<p>The <strong>creator economy</strong> is the combination of software and marketplaces that make it possible for creative people and teams to add content to the metaverse. This ranges from individual assets (e.g., a piece of artwork) up to an entire system (e.g., a game, virtual world, mod, or crafted experience).</p>

<p><a href="/virtual-world">Virtual economies</a> within <a href="/virtual-world">virtual worlds</a> may allow participants to craft individual virtual items, customize avatars, or even make entirely new mods or terrain.</p>

<p>One of the enablers of the creator economy is <a href="/low-code-platform">Low-Code Platforms</a>.</p>`,
				"topic_links": `<ul>
	<li><a href="/virtual-world">Virtual World</a></li>
	<li><a href="/low-code-platform">Low Code Platform</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/evolution-of-the-creator-economy-9f5a90cb7e92" target="_blank">Evolution of the Creator Economy</a></li>
	<li><a href="https://li.substack.com/p/legitimacy-lost" target="_blank">Legitimacy Lost</a> (Li Jin and Katie Parrott)</li>
	<li><a href="https://medium.com/@rfrkim/thoughts-at-the-intersection-of-web3-and-creative-culture-af39f9e88e18" target="_blank">Thoughts at the Intersection of Web3 and Creative Culture</a> (Richard Kim)</li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a> (discusses Fortnite, Minecraft, Decentraland)</li>
</ul>`,
			},
		},
		{
			Title:           "Megatrends",
			Slug:            "megatrends",
			TemplateSlug:    "concept-page",
			MetaDescription: "Megatrends are large, global trends that will impact the future.",
			Data: map[string]interface{}{
				"definition": `<p>Megatrends are large, global trends that will impact the future.</p>

<p>The megatrends shaping the metaverse are both social as well as technological:</p>
<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li>Cybernetics</li>
	<li><a href="/distributed-network">Distributed Network</a></li>
	<li><a href="/low-code-platform">Low Code Platforms</a></li>
	<li><a href="/artificial-intelligence">Machine Intelligence</a></li>
	<li>Open Platforms</li>
	<li>Simulating Reality</li>
	<li>Virtual Mainstreaming</li>
	<li>Walled Gardens</li>
</ul>

<p>Explore the 9 trends by clicking through to each topic, or read the in-depth article below to learn more.</p>`,
				"topic_links": `<ul>
	<li><a href="/market-layer">Market Layer</a></li>
	<li><a href="/infrastructure">Infrastructure</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a> (full article)</li>
</ul>`,
			},
		},
		{
			Title:           "Market Layer",
			Slug:            "market-layer",
			TemplateSlug:    "concept-page",
			MetaDescription: "A large number of companies are building the metaverse. These can be generally grouped into 7 conceptual categories.",
			Data: map[string]interface{}{
				"definition": `<p>A large number of companies are building the metaverse. These can be generally grouped into the following 7 conceptual categories:</p>
<ol>
	<li><strong><a href="/experiences">Experiences</a></strong>—what people do in the metaverse</li>
	<li><strong><a href="/discovery">Discovery</a></strong>—how people find things to do</li>
	<li><strong><a href="/creator-economy">Creator Economy</a></strong>—how people make things in the metaverse</li>
	<li><strong><a href="/spatial-computing">Spatial Computing</a></strong>—technology to render and interact with immersive space</li>
	<li><strong><a href="/decentralization">Decentralization</a></strong>—open source, open standards and blockchain</li>
	<li><strong><a href="/human-interface">Human Interface</a></strong>—hardware to access the metaverse</li>
	<li><strong><a href="/infrastructure">Infrastructure</a></strong>—enabling technology like networks, batteries, and semiconductors</li>
</ol>`,
				"topic_links": `<ul>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/infrastructure">Infrastructure</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a>—Expands on the 7 layers identified above.</li>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">Market Map of the Metaverse</a>—Groups 200+ companies across these 7 layers with comparisons of Roblox, Unity, and Epic</li>
</ul>`,
			},
		},
		{
			Title:           "Esports",
			Slug:            "esports",
			TemplateSlug:    "concept-page",
			MetaDescription: "Esports is a category of competitive gaming experiences.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Esports</strong> is a category of competitive gaming experiences within the metaverse.</p>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/immersive-social">Immersive Social</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/experiences-of-the-metaverse-2e8e09e1bbf7" target="_blank">Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		{
			Title:           "Experiences",
			Slug:            "experiences",
			TemplateSlug:    "concept-page",
			MetaDescription: "Experiences are what people do in the metaverse.",
			Data: map[string]interface{}{
				"definition": `<p>Experiences are what people <em>do</em> in the metaverse.</p>

<p>Examples include:</p>
<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/immersive-social">Immersive Social</a></li>
	<li>Future of Work</li>
	<li>Fitness</li>
	<li>Telepresence travel</li>
	<li>Live music</li>
	<li><a href="/esports">Esports</a></li>
</ul>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/immersive-social">Immersive Social</a></li>
	<li><a href="/esports">Esports</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/experiences-of-the-metaverse-2e8e09e1bbf7" target="_blank">Experiences of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89696" target="_blank">Market Map of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		{
			Title:           "Discovery",
			Slug:            "discovery",
			TemplateSlug:    "concept-page",
			MetaDescription: "Discovery is how people learn about the things they can do in the metaverse.",
			Data: map[string]interface{}{
				"definition": `<p>Discovery is how people learn about the things they can do in the metaverse.</p>

<p>This encompasses both established marketing channels (advertising networks, sponsorships, curated app stores) and community-focused alternatives (decentralized marketplaces, messaging platforms).</p>`,
				"topic_links": `<ul>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		{
			Title:           "Human Interface",
			Slug:            "human-interface",
			TemplateSlug:    "concept-page",
			MetaDescription: "Human Interfaces Hardware includes traditional screens as well as emergent technology such as virtual reality and augmented reality.",
			Data: map[string]interface{}{
				"definition": `<p>Human Interfaces Hardware includes traditional screens (on computers, phones, etc.) as well as emergent technology such as virtual reality and augmented reality—as well as futuristic technologies like brain-computer interfaces.</p>`,
				"topic_links": `<ul>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/market-layer">Market Layer</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
</ul>`,
			},
		},
	}

	// Combine all pages
	allPages := append(pages, conceptPages...)

	// Insert all pages
	for _, page := range allPages {
		templateID, ok := templates[page.TemplateSlug]
		if !ok {
			fmt.Printf("Template not found: %s for page %s\n", page.TemplateSlug, page.Title)
			continue
		}

		fullPath := "/" + page.Slug
		if page.Slug == "" {
			fullPath = "/"
		}

		// Check if content already exists
		count, _ := db.Collection("content").CountDocuments(ctx, bson.M{"full_path": fullPath})
		if count > 0 {
			fmt.Printf("Skipping (exists): %s\n", fullPath)
			continue
		}

		content := Content{
			TemplateID:      templateID,
			TemplateName:    page.TemplateSlug,
			Title:           page.Title,
			Slug:            page.Slug,
			FullPath:        fullPath,
			Category:        "",
			MetaDescription: page.MetaDescription,
			Data:            page.Data,
			Published:       true,
			PublishedAt:     &now,
			UseHeader:       true,
			UseFooter:       true,
			UseTheme:        true,
			RawMode:         false,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		result, err := db.Collection("content").InsertOne(ctx, content)
		if err != nil {
			fmt.Printf("Error inserting %s: %v\n", page.Title, err)
		} else {
			fmt.Printf("Created: %s (%s) -> %v\n", page.Title, fullPath, result.InsertedID)
		}
	}

	fmt.Println("\nMigration complete!")
}
