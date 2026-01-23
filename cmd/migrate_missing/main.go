package main

import (
	"context"
	"fmt"
	"log"
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

type Template struct {
	ID   primitive.ObjectID `bson:"_id"`
	Name string             `bson:"name"`
	Slug string             `bson:"slug"`
}

type PageContent struct {
	Title           string
	Slug            string
	TemplateSlug    string
	MetaDescription string
	Data            map[string]interface{}
}

func main() {
	mongoURI := dbutil.GetMongoURI()
	if mongoURI == "" {
		log.Fatal("MONGO_URI not set. Set it via environment variable or config file.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("lightcms")

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

	// Define missing pages - EXACT TEXT extracted from metavert.io
	pages := []PageContent{
		// Decentralized Finance
		{
			Title:           "Decentralized Finance",
			Slug:            "decentralized-finance",
			TemplateSlug:    "concept-page",
			MetaDescription: "DeFi applications use smart contracts on the blockchain to enable trustless, permissionless financial applications.",
			Data: map[string]interface{}{
				"definition": `<p>A class of applications on the <a href="/blockchain">blockchain</a> that use <a href="/smart-contract">smart contracts</a> to enable software to cooperate in a trustless, permissionless manner to implement new types of financial applications including decentralized exchanges, lending protocols, fractionalized ownership (such as for real estate or digital assets like NFTs). These applications do not depend on centralized authorities like banks or brokerages.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://101blockchains.com/smart-contracts/" target="_blank">Smart Contracts: the Ultimate Guide for Beginners</a></li>
	<li><a href="https://medium.com/@rkmjcharpb/a-defi-stream-of-consciousness-and-the-opportunity-for-trustless-games-a4f3b34cf0f0" target="_blank">A DeFi Stream of Consciousness and the opportunity for "trustless games"</a></li>
	<li><a href="https://medium.com/tellor/what-is-an-oracle-in-defi-and-why-does-it-matter-72abc801b5d9" target="_blank">What is an Oracle in DeFi and Why does it matter?</a></li>
</ul>`,
			},
		},
		// Avatar
		{
			Title:           "Avatar",
			Slug:            "avatar",
			TemplateSlug:    "concept-page",
			MetaDescription: "Avatars are a graphical representation of a person's digital identity in games and metaverse experiences.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Avatars</strong> are a graphical representation of a person's <a href="/digital-identity">digital identity</a>.</p>

<p>An avatar can be either a simple 2D representation (like profile pictures on social media) or a 3D representation (used in <a href="/games">games</a> and metaverse <a href="/experiences">experiences</a>). In interactive environments, avatars are often customizable with physical traits and <a href="/virtual-item">virtual items</a> including clothing and accessories.</p>`,
				"topic_links": `<ul>
	<li><a href="/digital-identity">Digital Identity</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/when-the-virtual-became-real-4168809879f5" target="_blank">When the Virtual Became Real</a></li>
	<li><a href="https://digitalnative.substack.com/p/stay-for-who-you-can-be-avatars-in" target="_blank">Stay for Who You Can Be</a></li>
	<li><a href="https://www.linkedin.com/pulse/how-brands-can-thrive-direct-avatar-economy-cathy-hackl/" target="_blank">How Brands Can Thrive in the Direct to Avatar Economy</a></li>
</ul>`,
			},
		},
		// Cryptocurrency
		{
			Title:           "Cryptocurrency",
			Slug:            "cryptocurrency",
			TemplateSlug:    "concept-page",
			MetaDescription: "A cryptocurrency is a form of virtual currency that uses cryptographic algorithms and blockchain.",
			Data: map[string]interface{}{
				"definition": `<p>A cryptocurrency is a form of <a href="/virtual-currency">virtual currency</a> that uses cryptographic algorithms and <a href="/blockchain">blockchain</a> to implement the classic functions of money: a store of value, unit of account and a medium of exchange. Popular examples include Bitcoin and Ethereum. Typically, cryptocurrencies are not backed by a centralized institution or government authority.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/virtual-currency">Virtual Currency</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="http://unenumerated.blogspot.com/2017/02/money-blockchains-and-social-scalability.html" target="_blank">Money, Blockchains and Social Scalability</a>, by Nick Szabo</li>
</ul>`,
			},
		},
		// Virtual Being
		{
			Title:           "Virtual Being",
			Slug:            "virtual-being",
			TemplateSlug:    "concept-page",
			MetaDescription: "A virtual being is a character or entity within a game or metaverse experience.",
			Data: map[string]interface{}{
				"definition": `<p>A virtual being is a character or other entity within a <a href="/games">game</a> or other metaverse <a href="/experiences">experience</a>. Virtual beings encompasses characters and avatars who are controlled by a human being (such as the 'vtubers' and livestreamers like Code Miko) who use a combination of mocap and 3D models to present themselves, as well as characters controlled by machine intelligence. The latter are an evolution of the concept of a non-player character (NPC)—characters in a story directed by the creators of a game or self-directed by AI.</p>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/experiences">Experiences</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-and-artificial-intelligence-ai-577343895411" target="_blank">The Metaverse and Artificial Intelligence</a></li>
	<li><a href="https://www.wired.com/story/get-wired-podcast-3-virtual-beings/" target="_blank">The Rise of the Virtual Being</a></li>
</ul>`,
			},
		},
		// Virtual Item
		{
			Title:           "Virtual Item",
			Slug:            "virtual-item",
			TemplateSlug:    "concept-page",
			MetaDescription: "A virtual item is an object possessed by a player in a game or metaverse experience.",
			Data: map[string]interface{}{
				"definition": `<p>A virtual item is an object possessed by a player in a <a href="/games">game</a> or other metaverse <a href="/experiences">experience</a>. Typically, you do not actually 'own' virtual items: you are granted a license to make use of them in whatever way the game or experience has decided.</p>

<p>Some <a href="/blockchain">blockchain</a>-based experiences that use <a href="/non-fungible-token">NFTs</a> are built to allow ownership of the item, including transferring the objects to other people. This may or may not grant additional rights, which are still determined by the creator.</p>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://www.gamedeveloper.com/business/types-of-game-currencies-in-mobile-free-to-play" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://www.playableworlds.com/news/riffs-by-raph:-how-virtual-worlds-work-part-1/" target="_blank">How Virtual Worlds Work</a>, series by Raph Koster</li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a></li>
</ul>`,
			},
		},
		// Virtual Economy
		{
			Title:           "Virtual Economy",
			Slug:            "virtual-economy",
			TemplateSlug:    "concept-page",
			MetaDescription: "Virtual Economies are systems that enable users to control virtual currency and virtual items within a virtual world.",
			Data: map[string]interface{}{
				"definition": `<p>Virtual Economies are systems that enable users to control virtual currency and virtual items within a <a href="/virtual-world">virtual world</a> or other metaverse experience.</p>

<p>Many free-to-play games incorporate economies allowing players to earn certain items while purchasing others with real money. Economies fall into three categories:</p>
<ul>
	<li><strong>Closed</strong> (examples include World of Warcraft and Fortnite, where third-party marketplaces are typically prohibited by terms of service)</li>
	<li><strong>Open and centralized</strong> (games like Minecraft permit <a href="/creator-economy">creator economy</a> participation within defined limits)</li>
	<li><strong>Open and <a href="/decentralization">decentralized</a></strong> (blockchain-based games such as Decentraland feature player-to-player exchange systems designed with openness in mind)</li>
</ul>`,
				"topic_links": `<ul>
	<li><a href="/virtual-world">Virtual World</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://www.gamedeveloper.com/business/types-of-game-currencies-in-mobile-free-to-play" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://www.playableworlds.com/news/riffs-by-raph:-how-virtual-worlds-work-part-1/" target="_blank">How Virtual Worlds Work</a></li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a></li>
</ul>`,
			},
		},
		// Web3
		{
			Title:           "Web3",
			Slug:            "web3",
			TemplateSlug:    "concept-page",
			MetaDescription: "Web3 is a collection of design patterns using peer-to-peer or blockchain-based platforms instead of centralized servers.",
			Data: map[string]interface{}{
				"definition": `<p>Web3 is a collection of design patterns and methods in which Web-based applications use peer-to-peer or blockchain-based platforms for storing data rather than centralized servers and <a href="/walled-garden">walled-garden</a> platforms.</p>

<p>A Web3 wallet is a browser plug-in that allows you to access <a href="/cryptocurrency">cryptocurrencies</a> and interact with <a href="/smart-contract">smart contracts</a> on web pages.</p>`,
				"topic_links": `<ul>
	<li><a href="/open-platform">Open Platform</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/walled-garden">Walled Garden</a></li>
	<li><a href="/cryptocurrency">Cryptocurrency</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://galaxyrtk.substack.com/p/thoughts-at-the-intersection-of-web3" target="_blank">Thoughts at the Intersection of Web3 and Creative Culture</a></li>
	<li><a href="https://docs.google.com/document/d/1SWJw_NTyUvgdB_asRzsnVyKjciW8dZbeqQeUeWsEiQc/edit" target="_blank">Introduction to Web3 and blockchain core concepts</a></li>
</ul>`,
			},
		},
		// Metaverse
		{
			Title:           "Metaverse",
			Slug:            "metaverse",
			TemplateSlug:    "concept-page",
			MetaDescription: "The metaverse is the next generation of the internet, oriented around real-time, immersive activity.",
			Data: map[string]interface{}{
				"definition": `<p>The metaverse is the next generation of the internet. It is better understood as a set of convergent trends than a 'product.'</p>

<p>Key trends include: real-time activity (versus more informational or transactional applications), more playful and immersive experiences—largely enabled by GameTech, powered by a <a href="/creator-economy">creator economy</a>, and linking, embedding and loose coupling via (re)<a href="/decentralization">decentralization</a>.</p>`,
				"topic_links": `<ul>
	<li><a href="/creator-economy">Creator Economy</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-canon-reading-guide-9eb1b371b505" target="_blank">The Metaverse Canon: a Reading Guide</a></li>
	<li><a href="https://medium.com/building-the-metaverse/what-we-talk-about-when-we-talk-about-the-metaverse-c9ef03c1a5dd" target="_blank">What we Talk About when We Talk About the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-experiences-of-the-metaverse-2126a7899020" target="_blank">Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		// Generative AI
		{
			Title:           "Generative AI",
			Slug:            "generativeai",
			TemplateSlug:    "concept-page",
			MetaDescription: "Generative AI involves generating various forms of content: text, images, levels for games, music, audio, and more.",
			Data: map[string]interface{}{
				"definition": `<p>Generative AI is a domain within <a href="/artificial-intelligence">artificial intelligence</a> that involves generating various forms of content: text, images (2D and 3D), levels for games, music, audio, speech, videos, etc.</p>

<p>These systems typically employ text prompts to convert natural language into specific outputs, resulting in designations like 'text-to-image' and 'text-to-speech.' When systems facilitate interactive dialogue, they're classified as chatbots or assistants.</p>

<p>The various text-to-output systems ordinarily use a 'large language model' (LLM) which is an AI that is trained to work with human language.</p>`,
				"topic_links": `<ul>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://metavert.substack.com/p/the-generative-ai-canon" target="_blank">The Generative AI Canon</a></li>
</ul>`,
			},
		},
		// Brain-Computer Interface
		{
			Title:           "Brain-Computer Interface",
			Slug:            "brain-computer-interface",
			TemplateSlug:    "concept-page",
			MetaDescription: "A brain-computer interface allows users to go directly from thought to a computer system.",
			Data: map[string]interface{}{
				"definition": `<p>A brain-computer interface (BCI) is a form of interface that allows users to go directly from thought to a computer system.</p>

<p>BCIs can be <strong>invasive</strong> (requiring electrodes or chips implanted in the brain) or <strong>noninvasive</strong> (using external sensors on the head or body).</p>

<p>In 2021, a BCI system designed for individuals unable to use their hands achieved over 18 words per minute typing speed. Potential applications include enhancing various interfaces, supplementing gesture recognition systems, and providing sensory feedback to users.</p>`,
				"topic_links": `<ul>
	<li><a href="/human-interface">Human Interface</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://spectrum.ieee.org/braincomputer-interface-smashes-previous-record-for-typing-speed" target="_blank">Brain-Computer Interface Smashes Previous Record for Typing Speed</a></li>
</ul>`,
			},
		},
		// Cybernetics
		{
			Title:           "Cybernetics",
			Slug:            "cybernetics",
			TemplateSlug:    "concept-page",
			MetaDescription: "Cybernetics is technology for bridging the gap between machines and the human sensorimotor systems.",
			Data: map[string]interface{}{
				"definition": `<p>Cybernetics is technology for bridging the gap between machines and the human sensorimotor systems. A game controller represents a basic form, while <a href="/virtual-reality">virtual reality</a> headsets, wearables, and smartglasses exemplify more advanced applications. Brain-computer interfaces (BCIs) are positioned as a future development in this field.</p>

<p>Cybernetics increasingly relies on breakthroughs in <a href="/artificial-intelligence">machine intelligence</a>, enabling progression from limited interfaces (text input) to more sophisticated, interpretive systems (speech and gesture recognition).</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
</ul>`,
			},
		},
		// Decentralized Autonomous Organization
		{
			Title:           "Decentralized Autonomous Organization",
			Slug:            "decentralized-autonomous-organization",
			TemplateSlug:    "concept-page",
			MetaDescription: "DAOs are a form of on-chain governance where token holders vote on proposals.",
			Data: map[string]interface{}{
				"definition": `<p>Decentralized Autonomous Organizations (DAOs) are a form of on-chain governance. Members who hold tokens in the DAO are able to vote on proposals that guide the direction of the organization.</p>

<p>DAOs apply to diverse scenarios: software projects, protocols, gaming guilds, homeowners associations, companies, NGOs, etc.</p>

<p>A key benefit is that it can bring software-driven governance systems to organizations that either lacked transparent, consistent or easy-to-implement governance before.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.youtube.com/watch?v=SYPzqRaN4zA" target="_blank">Decentralized Autonomous Organizations (DAOs)</a>, Stanford</li>
	<li><a href="https://docs.google.com/presentation/d/1fLJvPOvibcCUpJ9ES44_cdoX5Hb7LpDaloGWz5FbUEM/edit#slide=id.ged448f070e_4_425" target="_blank">DAOs — the New Frontier in Coordination</a></li>
	<li><a href="https://www.youtube.com/watch?v=MFEXFvCFywc" target="_blank">Blockchain Governance</a>, Coin Bureau</li>
</ul>`,
			},
		},
		// Digital Hologram
		{
			Title:           "Digital Hologram",
			Slug:            "digital-hologram",
			TemplateSlug:    "concept-page",
			MetaDescription: "A Digital Hologram simulates the projection of coherent images in virtual environments.",
			Data: map[string]interface{}{
				"definition": `<p>A hologram is the projection of a coherent image into physical space. At current technology levels, this is only possible within fixed hardware displays.</p>

<p>A <strong>Digital Hologram</strong> simulates this concept in virtual environments. It can involve projecting objects into <a href="/augmented-reality">augmented reality</a> experiences or creating user interfaces within virtual/AR spaces—potentially replacing traditional screens like phones and computers.</p>`,
				"topic_links": `<ul>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
</ul>`,
			},
		},
		// Digital Identity
		{
			Title:           "Digital Identity",
			Slug:            "digital-identity",
			TemplateSlug:    "concept-page",
			MetaDescription: "Digital Identity encompasses all the means through which you can identify yourself in online applications.",
			Data: map[string]interface{}{
				"definition": `<p>Digital Identity encompasses all the means through which you can identify yourself in online applications and express yourself within them. This includes:</p>
<ul>
	<li>Authentication software for logging into online services (banks, games, social networks)</li>
	<li>Software that projects your identity into environments, such as an <a href="/avatar">avatar</a></li>
	<li><a href="/virtual-item">Virtual items</a> associated with games or avatars</li>
	<li>Emerging <a href="/selfsovereign-identity">self-sovereign identity</a> technology enabling independent identification</li>
	<li><a href="/zero-knowledge-proofs">Zero-knowledge proofs</a> allowing selective information disclosure to applications</li>
</ul>`,
				"topic_links": `<ul>
	<li><a href="/virtual-mainstreaming">Virtual Mainstreaming</a></li>
	<li><a href="/avatar">Avatar</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
	<li><a href="/selfsovereign-identity">Self-Sovereign Identity</a></li>
	<li><a href="/zero-knowledge-proofs">Zero Knowledge Proofs</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/when-the-virtual-became-real-4168809879f5" target="_blank">When the Virtual Became Real</a></li>
	<li><a href="https://www.frontiersin.org/articles/10.3389/fbloc.2019.00028/full" target="_blank">Self-Sovereign Identity in a Globalized World</a></li>
</ul>`,
			},
		},
		// Digital Twin
		{
			Title:           "Digital Twin",
			Slug:            "digital-twin",
			TemplateSlug:    "concept-page",
			MetaDescription: "A digital twin is a digital object that mirrors the real-time properties of a physical object.",
			Data: map[string]interface{}{
				"definition": `<p>A digital twin is a digital object in the metaverse that mirrors the real-time properties of a physical object.</p>

<p>It may include its visual properties (e.g., a <a href="/3d-engine">3D representation</a> of the current state of the source) and/or include data feeds based on sensors from the source object.</p>

<p>For example, a refrigerator's digital twin might display a 3D model alongside sensor data about temperature, humidity, and door status.</p>`,
				"topic_links": `<ul>
	<li><a href="/simulating-reality">Simulating Reality</a></li>
	<li><a href="/3d-engine">3D Engine</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
</ul>`,
			},
		},
		// Free-to-Play
		{
			Title:           "Free-to-Play",
			Slug:            "freetoplay",
			TemplateSlug:    "concept-page",
			MetaDescription: "Free-to-play is a monetization model used by a large number of online games.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Free-to-play (F2P)</strong> is a monetization model used by a large number of online games. Players can start without payment but may purchase virtual items or currency for progression.</p>

<p>The model generates revenue through optional purchases and advertising, representing over half the game industry's revenue as of 2021.</p>

<p>The F2P model incorporates:</p>
<ul>
	<li><a href="/virtual-economy">Virtual economies</a> enabling free entry</li>
	<li><a href="/virtual-item">Virtual items</a> and currency (some premium-only)</li>
	<li>Real-money transactions as primary revenue source</li>
	<li>Ad-supported monetization</li>
</ul>`,
				"topic_links": `<ul>
	<li><a href="/virtual-economy">Virtual Economy</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/game-economics-part-3-free-to-play-a1b3d2c00db8" target="_blank">Game Economics Part 3: Free-to-play</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://www.gamedeveloper.com/business/types-of-game-currencies-in-mobile-free-to-play" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a></li>
</ul>`,
			},
		},
		// Future of Work
		{
			Title:           "Future of Work",
			Slug:            "future-of-work",
			TemplateSlug:    "concept-page",
			MetaDescription: "The Future of Work encompasses workforce collaboration without physical presence.",
			Data: map[string]interface{}{
				"definition": `<p>The Future of Work encompasses workforce collaboration without physical presence.</p>

<p>It includes current applications like video teleconferencing and platforms such as Gather.town, with potential expansion through <a href="/virtual-reality">virtual reality</a> or <a href="/augmented-reality">augmented reality</a> technologies.</p>`,
				"topic_links": `<ul>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-experiences-of-the-metaverse-2126a7899020" target="_blank">The Experiences of the Metaverse</a></li>
	<li><a href="https://chief.com/articles/metaverse-future-of-work" target="_blank">What You Need to Know About the Metaverse — and Why It Matters for Your Bottom Line</a></li>
	<li><a href="https://medium.com/building-the-metaverse/jobs-in-the-metaverse-9395db90086" target="_blank">Jobs of the Metaverse</a></li>
</ul>`,
			},
		},
		// 5G
		{
			Title:           "5G",
			Slug:            "glossary-5g",
			TemplateSlug:    "concept-page",
			MetaDescription: "5G is the fifth generation of wireless networks with improved latency, connections, and speed.",
			Data: map[string]interface{}{
				"definition": `<p>The fifth generation of wireless networks. 5G technology is an order-of-magnitude improvement in latency, number of concurrent connections, and overall speed.</p>

<p>The technology delivers enhanced performance in congested urban environments and supports real-time applications including gaming, video calling, and edge-computing tasks.</p>

<p>5G will eventually be superseded by 6G networks.</p>`,
				"topic_links": `<ul>
	<li><a href="/infrastructure">Infrastructure</a></li>
	<li><a href="/distributed-network">Distributed Network</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		// Graphics Processing Unit
		{
			Title:           "Graphics Processing Unit",
			Slug:            "graphics-processing-unit",
			TemplateSlug:    "concept-page",
			MetaDescription: "GPUs are specialized chips designed for parallel tasks including rendering 3D graphics and AI.",
			Data: map[string]interface{}{
				"definition": `<p>Graphics Processing Units (GPUs) are specialized chips designed for highly parallel tasks including rendering <a href="/3d-engine">3D graphics</a> and <a href="/artificial-intelligence">artificial intelligence</a>.</p>

<p>Since rendering <a href="/spatial-computing">spatial environments</a> and machine intelligence are key metaverse components, GPUs serve as fundamental enablers.</p>`,
				"topic_links": `<ul>
	<li><a href="/infrastructure">Infrastructure</a></li>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
	<li><a href="https://www.techspot.com/article/650-history-of-the-gpu/" target="_blank">History of the Modern Graphics Processor</a></li>
</ul>`,
			},
		},
		// Mods
		{
			Title:           "Mod",
			Slug:            "mod",
			TemplateSlug:    "concept-page",
			MetaDescription: "Mods are modifications to a game or virtual world.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Mods</strong> are "modifications" to a <a href="/games">game</a> or <a href="/virtual-world">virtual world</a>.</p>

<p><strong>Modding</strong> is the creative act of making mods.</p>

<p>Sometimes mods are made for free as a creative project; at other times, mods are sold as enhancements to a game. In the latter case, modding may be part of a <a href="/creator-economy">creator economy</a> around the game in question.</p>

<p>Some of the largest games in the world today began as mods: Counterstrike, PUBG and DOTA are three examples.</p>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/virtual-world">Virtual World</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.thegamer.com/pc-games-active-modding-communities/" target="_blank">PC Games With The Most Active Modding Communities</a></li>
</ul>`,
			},
		},
		// Network Effects
		{
			Title:           "Network Effects",
			Slug:            "network-effects",
			TemplateSlug:    "concept-page",
			MetaDescription: "Network Effects represent theories about how networks increase in value.",
			Data: map[string]interface{}{
				"definition": `<p>Network Effects represent various theories about how networks increase in value.</p>

<p><a href="https://medium.com/building-the-metaverse/network-effects-in-the-metaverse-5c39f9b94f5a" target="_blank">Metcalfe's Law</a> theorizes that the value of a network increases proportional to the square of its users.</p>

<p>Reed's Law suggests that large networks (including software applications like social networks) grow in value exponentially when their subgroups have reduced adoption friction.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/network-effects-in-the-metaverse-5c39f9b94f5a" target="_blank">Network Effects in the Metaverse</a></li>
	<li><a href="https://stratechery.com/2018/the-moat-map/" target="_blank">The Moat Map</a></li>
	<li><a href="https://www.nfx.com/post/network-effects-manual/" target="_blank">The Network Effects Manual</a></li>
	<li><a href="https://medium.com/mit-cryptoeconomics-lab/the-blockchain-effect-86bd01006ec2" target="_blank">The Blockchain Effect</a></li>
</ul>`,
			},
		},
		// Open Platform
		{
			Title:           "Open Platform",
			Slug:            "open-platform",
			TemplateSlug:    "concept-page",
			MetaDescription: "An open platform is a permissionless technology that allows creators to make content not tied to a walled garden.",
			Data: map[string]interface{}{
				"definition": `<p>An open platform is a <a href="/permissionless">permissionless</a> technology that allows creators to make content and applications that are not tied to a particular <a href="/walled-garden">walled garden</a>.</p>

<p>Creators benefit from decentralized ownership independent of corporate gatekeeping, though they often face steeper technical requirements and must build audiences themselves.</p>

<p>The definition encompasses a spectrum—from entirely public domain initiatives like GNU software to commercially-owned systems (Windows software development) that don't require proprietary permission. This also includes <a href="/decentralization">decentralized</a> applications using <a href="/smart-contract">smart contracts</a>, open standards like <a href="/openxr">OpenXR</a> and <a href="/wasm">WASM</a>, plus <a href="/web3">Web3</a> wallets.</p>`,
				"topic_links": `<ul>
	<li><a href="/walled-garden">Walled Garden</a></li>
	<li><a href="/permissionless">Permissionless</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/openxr">OpenXR</a></li>
	<li><a href="/wasm">WASM</a></li>
	<li><a href="/web3">Web3</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-permissionless-metaverse-658872a35da4" target="_blank">The Permissionless Metaverse</a></li>
	<li><a href="https://www.fortressofdoors.com/so-you-want-to-compete-with-roblox/" target="_blank">So You Want to Compete with Roblox</a></li>
</ul>`,
			},
		},
		// OpenXR
		{
			Title:           "OpenXR",
			Slug:            "openxr",
			TemplateSlug:    "concept-page",
			MetaDescription: "OpenXR is an open platform API specification for 3D, AR, and VR software.",
			Data: map[string]interface{}{
				"definition": `<p>OpenXR is an <a href="/open-platform">open platform</a> application programming interface (API) specification for the delivery of 3D, <a href="/augmented-reality">Augmented Reality</a> and <a href="/virtual-reality">Virtual Reality</a> software.</p>`,
				"topic_links": `<ul>
	<li><a href="/open-platform">Open Platform</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.khronos.org/registry/OpenXR/specs/1.0/html/xrspec.html" target="_blank">OpenXR Specification</a></li>
</ul>`,
			},
		},
		// Permissionless
		{
			Title:           "Permissionless",
			Slug:            "permissionless",
			TemplateSlug:    "concept-page",
			MetaDescription: "A permissionless technology operates without requiring authorization to use.",
			Data: map[string]interface{}{
				"definition": `<p>A permissionless technology operates without requiring authorization to use. Most such technologies are <a href="/decentralization">decentralized</a>.</p>

<p>Examples include creating software based on <a href="/open-platform">open platforms</a> and open ecosystems like PC software development, as well as many <a href="/blockchain">blockchains</a>.</p>

<p>This contrasts with permissioned environments (App Store, Steam, traditional financial networks) that function as gatekeepers, determining participation eligibility and typically charging higher fees.</p>`,
				"topic_links": `<ul>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/open-platform">Open Platform</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-permissionless-metaverse-658872a35da4" target="_blank">The Permissionless Metaverse</a></li>
</ul>`,
			},
		},
		// Play-to-Earn
		{
			Title:           "Play-to-Earn",
			Slug:            "playtoearn",
			TemplateSlug:    "concept-page",
			MetaDescription: "Play-to-Earn is an economic model used by some blockchain-based games.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Play-to-Earn (P2E)</strong> is an economic model used by some <a href="/blockchain">blockchain</a>-based <a href="/games">games</a>.</p>

<p>In the <a href="/virtual-economy">virtual economy</a> of a P2E game, players may earn <a href="/virtual-item">virtual items</a> or virtual currency by playing the game. These items and currency may be sold to others via <a href="/decentralization">decentralized</a> marketplaces.</p>

<p>P2E games frequently make use of <a href="/non-fungible-token">NFTs</a>.</p>

<p>The game developer typically generates revenue through the sale of items or currency to players, and may also take a revenue share from secondary-market sales.</p>`,
				"topic_links": `<ul>
	<li><a href="/virtual-economy">Virtual Economy</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/game-economics-part-2-digital-collectibles-and-nfts-6dc629012cfe" target="_blank">Game Economics Part 2: Digital Collectibles and NFTs</a></li>
	<li><a href="https://medium.com/building-the-metaverse/types-of-virtual-items-e12daa9580a2" target="_blank">Types of Virtual Items</a></li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a></li>
</ul>`,
			},
		},
		// Proof-of-Stake
		{
			Title:           "Proof-of-Stake",
			Slug:            "proof-of-stake",
			TemplateSlug:    "concept-page",
			MetaDescription: "Proof-of-stake is a method for blockchains to process transactions without energy-intensive cryptographic algorithms.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Proof-of-stake</strong> is a method for <a href="/blockchain">blockchains</a> to process transactions on their network without the computationally-expensive (and energy-intensive) cryptographic algorithms typical of Bitcoin and Ethereum.</p>

<p>Rather than having "miners" that work through cryptographic puzzles, nodes in proof-of-stake networks typically have validators who stake a <a href="/cryptocurrency">cryptocurrency</a> which they lose in the event they attempt to hack the network.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/cryptocurrency">Cryptocurrency</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.bitdegree.org/crypto/tutorials/proof-of-work-vs-proof-of-stake" target="_blank">Proof of Work vs Proof of Stake</a></li>
</ul>`,
			},
		},
		// Self-Sovereign Identity
		{
			Title:           "Self-Sovereign Identity",
			Slug:            "selfsovereign-identity",
			TemplateSlug:    "concept-page",
			MetaDescription: "Self-Sovereign Identity grants the individual user full control and ownership over their digital identity.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Self-Sovereign Identity</strong> is a form of <a href="/digital-identity">digital identity</a> that grants the individual user full control and ownership over the identity. It is not beholden to any centralized authority. Many implementations are being built on <a href="/blockchain">blockchain</a> technology.</p>

<p>Using <a href="/zero-knowledge-proofs">zero knowledge proofs</a>, the owner of a self-sovereign identity may choose to selectively disclose information to third-party applications that they wish to share.</p>`,
				"topic_links": `<ul>
	<li><a href="/digital-identity">Digital Identity</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/zero-knowledge-proofs">Zero Knowledge Proofs</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/when-the-virtual-became-real-4168809879f5" target="_blank">When the Virtual Became Real</a></li>
	<li><a href="https://www.frontiersin.org/articles/10.3389/fbloc.2019.00028/full" target="_blank">Self-Sovereign Identity in a Globalized World</a></li>
	<li><a href="https://hackernoon.com/eli5-zero-knowledge-proof-78a276db9eff" target="_blank">ELI5: Zero Knowledge Proof</a></li>
</ul>`,
			},
		},
		// Simulating Reality
		{
			Title:           "Simulating Reality",
			Slug:            "simulating-reality",
			TemplateSlug:    "concept-page",
			MetaDescription: "Simulating Reality is a megatrend involving the ability to accurately simulate the real world within computers.",
			Data: map[string]interface{}{
				"definition": `<p>Simulating Reality is a technological megatrend involving the ability to accurately simulate or mirror the 'real world' within computers.</p>

<p>This development emerges from convergence of multiple technologies including <a href="/3d-engine">3D Engines</a>, physics-based modeling such as <a href="/ray-tracing">ray tracing</a>, <a href="/artificial-intelligence">machine intelligence</a> and access to <a href="/digital-twin">digital twins</a> containing information about people, processes and things from physical reality.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/3d-engine">3D Engine</a></li>
	<li><a href="/ray-tracing">Ray Tracing</a></li>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/digital-twin">Digital Twin</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
</ul>`,
			},
		},
		// Smartglasses
		{
			Title:           "Smartglasses",
			Slug:            "smartglasses",
			TemplateSlug:    "concept-page",
			MetaDescription: "Smartglasses are augmented reality headsets with speakers, microphones, cameras, and hologram projection.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Smartglasses</strong> are <a href="/augmented-reality">augmented reality</a> headsets that combine speakers for audio, microphones for recording and responding to voice commands, cameras for observing and recording the environment, along with the ability to project <a href="/digital-hologram">digital holograms</a> into your view of physical space.</p>

<p>Current smartglasses are generally considered to be too heavy, lacking in ergonomics and battery life for mass-market adoption. The semiconductors, material science, edge computing and batteries required to perfect smartglasses is a major area of <a href="/infrastructure">infrastructure</a> investment.</p>`,
				"topic_links": `<ul>
	<li><a href="/human-interface">Human Interface</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/digital-hologram">Digital Hologram</a></li>
	<li><a href="/infrastructure">Infrastructure</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.arm.com/blogs/blueprint/xr-ar-vr-mr-difference" target="_blank">xR, AR, VR, MR: What's the Difference in Reality?</a></li>
	<li><a href="https://www.techbriefs.com/component/content/article/tb/supplements/pit/features/technology-leaders/38622" target="_blank">The Future of Smart Glasses</a></li>
</ul>`,
			},
		},
		// Telepresence
		{
			Title:           "Telepresence",
			Slug:            "telepresence",
			TemplateSlug:    "concept-page",
			MetaDescription: "Telepresence is digital teleportation via augmented reality or virtual reality.",
			Data: map[string]interface{}{
				"definition": `<p>Telepresence is digital teleportation: the ability to travel to a remote physical location (or a simulated virtual location) via <a href="/augmented-reality">augmented reality</a> or <a href="/virtual-reality">virtual reality</a>.</p>

<p>Near-term applications include attending live music concerts, <a href="/esports">esports</a> events, and work collaboration. Longer-term possibilities could encompass virtual travel experiences to remote locations using drones or human guides.</p>`,
				"topic_links": `<ul>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
	<li><a href="/esports">Esports</a></li>
	<li><a href="/experiences">Experiences</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-experiences-of-the-metaverse-2126a7899020" target="_blank">The Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		// Virtual Currency
		{
			Title:           "Virtual Currency",
			Slug:            "virtual-currency",
			TemplateSlug:    "concept-page",
			MetaDescription: "A virtual currency functions as a unit of account for some aspect of a virtual economy.",
			Data: map[string]interface{}{
				"definition": `<p>A virtual currency functions as a unit of account for some aspect of a <a href="/virtual-economy">virtual economy</a>. World of Warcraft gold is an example, which players earn and use to purchase <a href="/virtual-item">virtual items</a> within that closed ecosystem.</p>

<p>Importantly, users typically don't own virtual currency outright but rather receive a license to make use of the currency in whatever way the game or experience has decided.</p>

<p>Some newer systems leverage <a href="/blockchain">blockchain</a> technology and cryptocurrency, which can be traded through <a href="/decentralization">decentralized</a> exchanges.</p>`,
				"topic_links": `<ul>
	<li><a href="/virtual-economy">Virtual Economy</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.gamedeveloper.com/business/types-of-game-currencies-in-mobile-free-to-play" target="_blank">Types of game currencies in mobile free-to-play</a></li>
	<li><a href="https://www.playableworlds.com/news/riffs-by-raph:-how-virtual-worlds-work-part-1/" target="_blank">How Virtual Worlds Work</a></li>
	<li><a href="https://atelier.net/virtual-economy/" target="_blank">The Virtual Economy</a></li>
</ul>`,
			},
		},
		// Virtual Mainstreaming
		{
			Title:           "Virtual Mainstreaming",
			Slug:            "virtual-mainstreaming",
			TemplateSlug:    "concept-page",
			MetaDescription: "Virtual Mainstreaming describes the increasing societal acceptance of digital identity and virtual property.",
			Data: map[string]interface{}{
				"definition": `<p>Consider how much more of your identity today is defined by who you are online, compared to earlier times in the past; and consider that we now have at least a couple generations of humans who have grown up in a world with online games, virtual items, cryptocurrency, esports and other forms of digital existence.</p>

<p>Virtual Mainstreaming describes the increasing societal acceptance of <a href="/digital-identity">digital identity</a> and <a href="/virtual-item">virtual property</a> as equivalent to—or potentially surpassing—their physical counterparts in importance.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/digital-identity">Digital Identity</a></li>
	<li><a href="/virtual-item">Virtual Item</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/when-the-virtual-became-real-4168809879f5" target="_blank">When the Virtual Became Real</a></li>
</ul>`,
			},
		},
		// Walled Garden
		{
			Title:           "Walled Garden",
			Slug:            "walled-garden",
			TemplateSlug:    "concept-page",
			MetaDescription: "A walled garden is a domain within the metaverse with specific rules, permissions, and tools.",
			Data: map[string]interface{}{
				"definition": `<p>A walled garden is a domain within the metaverse that allows people to create content according to a specific set of rules, permissions and tools.</p>

<p>These environments typically offer user-friendly tools and access to substantial audiences. However, platforms generally retain a significant portion of creator revenue in exchange for these benefits.</p>

<p>Most prominent Web 2.0 platforms operate as walled gardens and are expected to maintain power during the metaverse era. Their counterparts are <a href="/open-platform">open platforms</a>.</p>`,
				"topic_links": `<ul>
	<li><a href="/megatrends">Megatrends</a></li>
	<li><a href="/open-platform">Open Platform</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-permissionless-metaverse-658872a35da4" target="_blank">The Permissionless Metaverse</a></li>
	<li><a href="https://www.fortressofdoors.com/so-you-want-to-compete-with-roblox/" target="_blank">So You Want to Compete with Roblox</a></li>
</ul>`,
			},
		},
		// WASM
		{
			Title:           "WebAssembly (WASM)",
			Slug:            "wasm",
			TemplateSlug:    "concept-page",
			MetaDescription: "WebAssembly is an open standard for delivering binary executable code to web browsers.",
			Data: map[string]interface{}{
				"definition": `<p>WebAssembly (WASM) is an open standard for delivering binary executable code to web browsers.</p>

<p>It would allow for an open application environment that parallels the App Store environments on mobile phones—but built around standards and without a centralized <a href="/walled-garden">walled garden</a>.</p>`,
				"topic_links": `<ul>
	<li><a href="/open-platform">Open Platform</a></li>
	<li><a href="/walled-garden">Walled Garden</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://webassembly.org/" target="_blank">WebAssembly.org</a></li>
</ul>`,
			},
		},
		// Zero Knowledge Proofs
		{
			Title:           "Zero Knowledge Proofs",
			Slug:            "zero-knowledge-proofs",
			TemplateSlug:    "concept-page",
			MetaDescription: "Zero Knowledge Proofs are algorithms that allow confirming information without disclosing additional details.",
			Data: map[string]interface{}{
				"definition": `<p>Zero Knowledge Proofs are algorithms that allow two parties to confirm a specific piece of information but without the grantor in the process disclosing any additional information.</p>

<p>For example, when proving your age at a bar, a zero knowledge proof would let you demonstrate you're over 21 without revealing your actual age, name, address, or other personal details from an ID.</p>`,
				"topic_links": `<ul>
	<li><a href="/digital-identity">Digital Identity</a></li>
	<li><a href="/selfsovereign-identity">Self-Sovereign Identity</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://hackernoon.com/eli5-zero-knowledge-proof-78a276db9eff" target="_blank">ELI5: Zero Knowledge Proof</a></li>
</ul>`,
			},
		},
		// DAOist
		{
			Title:           "DAOist",
			Slug:            "daoist",
			TemplateSlug:    "concept-page",
			MetaDescription: "A DAOist is a person who holds membership in or contributes to a Decentralized Autonomous Organization.",
			Data: map[string]interface{}{
				"definition": `<p>A <strong>DAOist</strong> is a person who holds membership in or contributes to a <a href="/decentralized-autonomous-organization">Decentralized Autonomous Organization (DAO)</a>.</p>

<p>DAOists participate in on-chain governance by holding tokens that allow them to vote on proposals guiding the direction of the organization. They may be involved in diverse scenarios including software projects, protocols, gaming guilds, homeowners associations, companies, or NGOs.</p>`,
				"topic_links": `<ul>
	<li><a href="/decentralized-autonomous-organization">Decentralized Autonomous Organization</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.youtube.com/watch?v=SYPzqRaN4zA" target="_blank">Decentralized Autonomous Organizations (DAOs)</a>, Stanford</li>
	<li><a href="https://docs.google.com/presentation/d/1fLJvPOvibcCUpJ9ES44_cdoX5Hb7LpDaloGWz5FbUEM/edit#slide=id.ged448f070e_4_425" target="_blank">DAOs — the New Frontier in Coordination</a></li>
</ul>`,
			},
		},
		// Deep Learning
		{
			Title:           "Deep Learning",
			Slug:            "deep-learning",
			TemplateSlug:    "concept-page",
			MetaDescription: "Deep learning is a subset of machine learning using artificial neural networks with multiple layers.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Deep learning</strong> is a subset of <a href="/artificial-intelligence">machine learning</a> that uses artificial neural networks with multiple layers to progressively extract higher-level features from raw input.</p>

<p>Deep Learning Transformers represent a major innovation in <a href="/artificial-intelligence">artificial intelligence</a>. The original Generative Pre-trained Transformer (GPT) worked with 110 million parameters; newer transformers work with over 1 trillion parameters.</p>

<p>Deep learning powers much of the metaverse including <a href="/spatial-computing">spatial computing</a>, creator tools, and sophisticated forms of storytelling through <a href="/generativeai">generative AI</a>.</p>`,
				"topic_links": `<ul>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/generativeai">Generative AI</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://www.youtube.com/watch?v=aircAruvnKk" target="_blank">Deep Learning in 5 Minutes</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-and-artificial-intelligence-ai-577343895411" target="_blank">The Metaverse and Artificial Intelligence</a></li>
</ul>`,
			},
		},
		// Esports
		{
			Title:           "Esports",
			Slug:            "esports-1",
			TemplateSlug:    "concept-page",
			MetaDescription: "Esports are the fusion between traditional sporting competitions and the world of digital games.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Esports</strong> are the fusion between traditional sporting competitions (live spectating, tournaments, competitions, professional teams, etc.) and the world of digital <a href="/games">games</a>.</p>

<p>Live esports tournaments increasingly occur in traditional stadium settings. As the metaverse develops, these venues are expected to integrate <a href="/augmented-reality">augmented reality</a> enhancements, with spectators potentially attending virtual events through <a href="/telepresence">telepresence</a> technology—similar to emerging practices in live entertainment broadcasting.</p>`,
				"topic_links": `<ul>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/telepresence">Telepresence</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://hir.harvard.edu/esports-part-1-what-are-esports/" target="_blank">Harvard International Review 4-part review on Esports</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-experiences-of-the-metaverse-2126a7899020" target="_blank">The Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		// Gamification
		{
			Title:           "Gamification",
			Slug:            "gamification",
			TemplateSlug:    "concept-page",
			MetaDescription: "Gamification is the application of game design principles to non-game contexts.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Gamification</strong> is the application of game design principles to non-game contexts. Points are important. Badges can be helpful. Leaderboards are compelling. But these are simply the tools of game design: they don't tell you what makes <a href="/games">games</a> actually work.</p>

<p>The problem with gamification isn't the term, or its objectives, but how it is applied. It is often caught up in the myth that games are nothing more than "Skinner boxes"—simple stimulus-response systems based on behaviorist psychology.</p>

<p>The metaverse delivers real gamification by layering in the more challenging aspects of game <a href="/experiences">experiences</a>: immersion, cooperation, and competition.</p>`,
				"topic_links": `<ul>
	<li><a href="/games">Games</a></li>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/metaverse">Metaverse</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://meditations.metavert.io/p/gamification-behaviorism-and-bullshit-50fe87861239" target="_blank">Gamification, Behaviorism and Bullshit</a></li>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-is-real-gamification-bc215fb4250b" target="_blank">The Metaverse is Real Gamification</a></li>
</ul>`,
			},
		},
		// Generative Art
		{
			Title:           "Generative Art",
			Slug:            "generative-art",
			TemplateSlug:    "concept-page",
			MetaDescription: "Generative art refers to art created using autonomous systems, algorithms, or artificial intelligence.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Generative art</strong> refers to art created in whole or in part using autonomous systems, algorithms, or <a href="/artificial-intelligence">artificial intelligence</a>.</p>

<p>In the metaverse context, generative art includes AI-generated images, procedurally created game content, and collaborative human-machine creative works. Tools like text-to-image systems allow creators to generate art from natural language prompts.</p>

<p>When players hit key milestones in a <a href="/games">game</a>, they could be invited to make generative art prompted by their unique experience of the story—creating shareable, memorable vignettes.</p>`,
				"topic_links": `<ul>
	<li><a href="/artificial-intelligence">Artificial Intelligence</a></li>
	<li><a href="/generativeai">Generative AI</a></li>
	<li><a href="/games">Games</a></li>
	<li><a href="/creator-economy">Creator Economy</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://meditations.metavert.io/p/generative-art-assets-for-games" target="_blank">Generative Art Assets for Games</a></li>
	<li><a href="https://meditations.metavert.io/p/market-map-generative-ai-for-virtual-worlds-efde3984e538" target="_blank">Market Map: Generative AI for Virtual Worlds</a></li>
</ul>`,
			},
		},
		// Geospatial Mapping
		{
			Title:           "Geospatial Mapping",
			Slug:            "geospatial-mapping",
			TemplateSlug:    "concept-page",
			MetaDescription: "Geospatial mapping is the process of mapping and interpreting the world for spatial computing applications.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Geospatial mapping</strong> is the process of mapping and interpreting the inside and outside world for use in <a href="/spatial-computing">spatial computing</a> applications.</p>

<p>It enables <a href="/augmented-reality">augmented reality</a> experiences to understand and interact with physical environments. Examples include Niantic Planet-Scale AR and Cesium for object recognition.</p>

<p>Geospatial mapping visualizes data linked to physical spaces in user environments, enabling the blend between physical and virtual worlds in the metaverse.</p>`,
				"topic_links": `<ul>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/digital-twin">Digital Twin</a></li>
	<li><a href="/simulating-reality">Simulating Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		// Haptics
		{
			Title:           "Haptics",
			Slug:            "haptics",
			TemplateSlug:    "concept-page",
			MetaDescription: "Haptics is the use of motion and vibration to simulate touch in digital experiences.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Haptics</strong> (or haptic feedback) is the use of motion and vibration to simulate touch. While most current technological devices predominantly stimulate vision and hearing, with haptics, machines can reach out and touch their users.</p>

<p>Haptic feedback communicates with users through sensory experience of touch, vibrations, motions, or the perceived application of force and pressure. It recreates the way we interact with the world around us.</p>

<p>Advances in haptics allow information to be transmitted through touch, even in the absence of physical objects. This enables control of virtual objects without having to touch them physically.</p>`,
				"topic_links": `<ul>
	<li><a href="/human-interface">Human Interface</a></li>
	<li><a href="/cybernetics">Cybernetics</a></li>
	<li><a href="/virtual-reality">Virtual Reality</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
	<li><a href="https://www.xrtoday.com/mixed-reality/what-is-haptic-feedback/" target="_blank">What Is Haptic Feedback?</a></li>
</ul>`,
			},
		},
		// Ideas
		{
			Title:           "Ideas",
			Slug:            "ideas",
			TemplateSlug:    "concept-page",
			MetaDescription: "The Ideas section presents foundational metaverse education through video overviews and analysis.",
			Data: map[string]interface{}{
				"definition": `<p>The <strong>Ideas</strong> section presents foundational metaverse education through multiple formats including video overviews, market maps, and analysis of megatrends.</p>

<p>The <a href="/metaverse">Metaverse</a> Market Map organizes approximately 200 projects and companies around 7 categories within the metaverse ecosystem. The 9 Megatrends provide an overview of major developmental trends shaping the future.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/cybernetics">Cybernetics</a></li>
	<li><a href="/distributed-network">Distributed Network</a></li>
	<li><a href="/low-code-platform">Low Code Platforms</a></li>
	<li><a href="/artificial-intelligence">Machine Intelligence</a></li>
	<li><a href="/open-platform">Open Platforms</a></li>
	<li><a href="/simulating-reality">Simulating Reality</a></li>
	<li><a href="/virtual-mainstreaming">Virtual Mainstreaming</a></li>
	<li><a href="/walled-garden">Walled Gardens</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/market-map-of-the-metaverse-8ae0cde89571" target="_blank">Market Map of the Metaverse</a></li>
	<li><a href="https://medium.com/building-the-metaverse/9-megatrends-shaping-the-metaverse-93b91c159375" target="_blank">9 Megatrends Shaping the Metaverse</a></li>
</ul>`,
			},
		},
		// Internet of Things
		{
			Title:           "Internet of Things",
			Slug:            "internet-of-things",
			TemplateSlug:    "concept-page",
			MetaDescription: "The Internet of Things refers to physical things embedded with sensors to exchange data over a network.",
			Data: map[string]interface{}{
				"definition": `<p>The <strong>Internet of Things (IoT)</strong> refers to physical things that are embedded with sensors, processing ability, and control software to enable them to exchange data over a network or the Internet.</p>

<p>IoT is a network of physical devices, automobiles, buildings, and other items that are linked to the internet and capable of collecting and sharing data. These devices have sensors, actuators, and communication capabilities that allow them to gather and transfer data in real time.</p>

<p>Data integration from IoT devices provides input for <a href="/spatial-computing">spatial computing</a> and <a href="/digital-twin">digital twin</a> applications in the metaverse.</p>`,
				"topic_links": `<ul>
	<li><a href="/infrastructure">Infrastructure</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
	<li><a href="/digital-twin">Digital Twin</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		// Media
		{
			Title:           "Media",
			Slug:            "media",
			TemplateSlug:    "concept-page",
			MetaDescription: "The Media section serves as a hub for Metavert's content distribution channels.",
			Data: map[string]interface{}{
				"definition": `<p>The <strong>Media</strong> section serves as a hub for Metavert's content distribution channels and social media presence.</p>

<p>Content is distributed across multiple platforms including Substack (Metavert Meditations newsletter), YouTube (Building the Metaverse channel), podcasts, and various social media channels.</p>`,
				"topic_links": `<ul>
	<li><a href="/metaverttv">Metavert.TV</a></li>
	<li><a href="/projects">Projects</a></li>
	<li><a href="/contact">Contact</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://metavert.substack.com" target="_blank">Metavert Meditations (Substack)</a></li>
	<li><a href="https://www.youtube.com/channel/UCZRJ0edG6flQiTpi3JgWX-w" target="_blank">Building the Metaverse YouTube Channel</a></li>
	<li><a href="https://anchor.fm/metaverse-radoff" target="_blank">Building the Metaverse Podcast</a></li>
</ul>`,
			},
		},
		// Mocap
		{
			Title:           "Mocap",
			Slug:            "mocap",
			TemplateSlug:    "concept-page",
			MetaDescription: "Motion capture (mocap) is the process of recording high-resolution movement into a computer system.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Motion capture</strong> (mocap or mo-cap) is the process of recording high-resolution movement of objects or people into a computer system. It is used in entertainment, sports, medical applications, and the metaverse.</p>

<p>In films, television shows, video games, and metaverse <a href="/experiences">experiences</a>, motion capture records actions of human actors and uses that information to animate digital character models in 2D or 3D computer animation. When it includes face and fingers or captures subtle expressions or voices, it is often referred to as performance capture.</p>

<p>Platforms like Kinetix democratize the creation of character animations, allowing individual creators to capture movement and make their <a href="/avatar">avatar</a> animations available through marketplaces.</p>`,
				"topic_links": `<ul>
	<li><a href="/avatar">Avatar</a></li>
	<li><a href="/experiences">Experiences</a></li>
	<li><a href="/human-interface">Human Interface</a></li>
	<li><a href="/spatial-computing">Spatial Computing</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://meditations.metavert.io/p/the-experiences-of-the-metaverse-2126a7899020" target="_blank">The Experiences of the Metaverse</a></li>
</ul>`,
			},
		},
		// Permissionless (alternate slug)
		{
			Title:           "Permissionless",
			Slug:            "permissionless-1",
			TemplateSlug:    "concept-page",
			MetaDescription: "A permissionless system is one that does not require permission to participate.",
			Data: map[string]interface{}{
				"definition": `<p>A <strong>permissionless</strong> system is one that does not require permission to participate. Examples include certain <a href="/open-platform">open platforms</a>, many <a href="/blockchain">blockchain</a> protocols, and most <a href="/decentralization">decentralized</a> systems; this is in contrast to <a href="/walled-garden">walled gardens</a> that control who is able to participate and what they are allowed to do.</p>`,
				"topic_links": `<ul>
	<li><a href="/open-platform">Open Platform</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/walled-garden">Walled Garden</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-permissionless-metaverse-658872a35da4" target="_blank">The Permissionless Metaverse</a></li>
	<li><a href="https://cdixon.org/2018/02/18/why-decentralization-matters" target="_blank">Why Decentralization Matters</a>, by Chris Dixon</li>
</ul>`,
			},
		},
		// Phygital
		{
			Title:           "Phygital",
			Slug:            "phygital",
			TemplateSlug:    "concept-page",
			MetaDescription: "Phygital refers to the integration of digital functionalities within physical experiences.",
			Data: map[string]interface{}{
				"definition": `<p><strong>Phygital</strong> is a portmanteau of "physical" and "digital." It refers to the integration of digital functionalities within physical experiences—a process that creates a hybrid experience bridging the digital world and physical reality.</p>

<p>By definition, the <a href="/metaverse">metaverse</a> is a phygital experience in itself. It combines real and virtual experiences, providing unique and interactive experiences for users.</p>

<p>Phygital Products combine physical-digital offerings whose components are linked together using <a href="/non-fungible-token">NFTs</a>, enabling new retail and distribution models.</p>`,
				"topic_links": `<ul>
	<li><a href="/metaverse">Metaverse</a></li>
	<li><a href="/augmented-reality">Augmented Reality</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
	<li><a href="/digital-twin">Digital Twin</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="https://medium.com/building-the-metaverse/the-metaverse-value-chain-afcf9e09e3a7" target="_blank">The Metaverse Value-Chain</a></li>
</ul>`,
			},
		},
		// Trustless
		{
			Title:           "Trustless",
			Slug:            "trustless",
			TemplateSlug:    "concept-page",
			MetaDescription: "A trustless system is one where parties can interact without needing to trust each other or an intermediary.",
			Data: map[string]interface{}{
				"definition": `<p>A <strong>trustless</strong> system is one where parties can interact without needing to trust each other or a third-party intermediary. <a href="/blockchain">Blockchain</a> technology enables trustless transactions through cryptographic verification and consensus mechanisms.</p>

<p><a href="/smart-contract">Smart contracts</a> are described as "autonomous, trustless, secure, transparent contracts" where counterparties may be anonymous, with use cases including record storage, title to assets, governance, insurance, and supply-chain automation.</p>

<p><a href="/non-fungible-token">NFTs</a> provide decentralized, trustless, programmable asset ownership and exchange with provable scarcity and provable provenance.</p>`,
				"topic_links": `<ul>
	<li><a href="/blockchain">Blockchain</a></li>
	<li><a href="/smart-contract">Smart Contract</a></li>
	<li><a href="/decentralization">Decentralization</a></li>
	<li><a href="/non-fungible-token">Non-Fungible Token</a></li>
</ul>`,
				"further_reading": `<ul>
	<li><a href="http://unenumerated.blogspot.com/2017/02/money-blockchains-and-social-scalability.html" target="_blank">Money, Blockchains and Social Scalability</a>, by Nick Szabo</li>
	<li><a href="https://101blockchains.com/smart-contracts/" target="_blank">Smart Contracts: the Ultimate Guide for Beginners</a></li>
</ul>`,
			},
		},
	}

	// Insert pages that don't already exist
	for _, page := range pages {
		templateID, ok := templates[page.TemplateSlug]
		if !ok {
			fmt.Printf("Template not found: %s for page %s\n", page.TemplateSlug, page.Title)
			continue
		}

		fullPath := "/" + page.Slug

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

	fmt.Println("\nMigration of missing pages complete!")
}
