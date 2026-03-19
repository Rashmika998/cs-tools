// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Button, Stack, Typography, Grid, Card } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Header as HeaderUI } from "@wso2/oxygen-ui";
import Brand from "@components/common/header/Brand";
import Actions from "@components/common/header/Actions";
import Footer from "@components/common/footer/Footer";
import styles from "./HomePage.module.css";

const ArrowIcon = (): JSX.Element => (
  <svg
    className={styles.cSVGarrow}
    xmlns="http://www.w3.org/2000/svg"
    version="1.1"
    viewBox="0 0 22.1 22.5"
    aria-hidden="true"
  >
    <path
      d="M21.7,10.2L12,.4c-.6-.6-1.6-.6-2.2,0-.6.6-.6,1.6,0,2.2l7.1,7.1H0v3.1h16.9l-7.1,7.1c-.6.6-.6,1.6,0,2.2.6.6,1.6.6,2.2,0l9.7-9.7c.6-.6.6-1.6,0-2.2Z"
      fill="currentColor"
    />
  </svg>
);

const featureItems = [
  {
    number: "01",
    title: "Speed-to-Market",
    description: "Get your solutions deployed faster with expert guidance and streamlined support processes.",
  },
  {
    number: "02",
    title: "Reliable Expertise",
    description: "Access to certified WSO2 engineers with deep product knowledge and industry experience.",
  },
  {
    number: "03",
    title: "Global Coverage",
    description: "Round-the-clock support across all time zones ensuring your business never stops.",
  },
];


export default function HomePage(): JSX.Element {
  return (
    <Box className={styles.pageWrapper}>
      {/* Navigation Header */}
      <HeaderUI>
        <Brand isNavigationDisabled={true} />
        <HeaderUI.Spacer />
        <Actions showUserProfile={false} />
      </HeaderUI>

      {/* Hero Section */}
      <Box className={styles.heroSection}>
        <Stack className={styles.heroContent}>
          {/* Hero Title */}
          <Typography variant="h1" className={styles.heroTitle}>
            Intelligent. Reliable. Expert.
          </Typography>

          {/* Hero Description */}
          <Typography variant="body1" className={styles.heroDescription}>
            World-class support to help you succeed. Get expert assistance, comprehensive resources, and a global community backing your WSO2 journey.
          </Typography>

          {/* CTA Buttons */}
          <div className={styles.ctaGroup}>
            <Button
              variant="contained"
              color="warning"
              component="a"
              href="#"
              target="_blank"
              rel="noopener noreferrer"
              endIcon={<ArrowIcon />}
              sx={{ textTransform: "none", fontSize: "1rem", fontWeight: 600, paddingX: 3, paddingY: 1.25 }}
            >
              Create Support Ticket
            </Button>

            <Button
              variant="outlined"
              color="warning"
              component="a"
              href="#"
              target="_blank"
              rel="noopener noreferrer"
              endIcon={<ArrowIcon />}
              sx={{ textTransform: "none", fontSize: "1rem", fontWeight: 600, paddingX: 3, paddingY: 1.25, borderColor: "rgba(255,255,255,0.4)", color: "#ffffff", "&:hover": { borderColor: "#ff6700", color: "#ff6700", backgroundColor: "rgba(255,103,0,0.08)" } }}
            >
              Browse Resources
            </Button>
          </div>
        </Stack>
      </Box>

      {/* Tiles Section — 2nd blade */}
      <Box className={styles.tilesSection}>
        <Box className={styles.tilesInner}>
          {/* Section Title */}
          <Typography variant="h2" className={styles.sectionTitle}>
            Explore Resources
          </Typography>

          {/* Tiles Grid */}
          <Grid container spacing={3}>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://docs.wso2.com/" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Documentation</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Comprehensive guides and API references</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/community" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Community</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Connect with WSO2 users worldwide</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/library" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Knowledge Base</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Solutions to common issues</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/training" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Onboarding</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Step-by-step guides to get started</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/subscription" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Support Policy</Typography>
                <Typography variant="body2" className={styles.cardDescription}>SLA commitments and procedures</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/subscription" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Subscription</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Manage licenses and upgrades</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://updates.docs.wso2.com/en/latest/" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>WSO2 Updates</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Latest product releases and patches</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, sm: 6, md: 3 }}>
              <Card component="a" href="https://wso2.com/products/support-matrix/" target="_blank" rel="noopener noreferrer" className={styles.card}>
                <Typography variant="h3" className={styles.cardTitle}>Support Lifecycle</Typography>
                <Typography variant="body2" className={styles.cardDescription}>Product support timelines and EOL</Typography>
                <Box className={styles.cardReadMore}>Read more <ArrowIcon /></Box>
              </Card>
            </Grid>
          </Grid>
        </Box>
      </Box>

      {/* Features + CTA Section — 3rd blade */}
      <Box className={styles.featuresSection}>
        <Box className={styles.featuresInner}>
          {/* Left: feature list */}
          <Box className={styles.featuresList}>
            <Typography variant="h2" className={styles.featuresTitle}>
              Speed-to-Market, Value, and Confidence Through Innovation
            </Typography>
            <Typography variant="body1" className={styles.featuresSubtitle}>
             WSO2 Support delivers enterprise-grade assistance that keeps your business running smoothly and efficiently.
            </Typography>
            <Stack spacing={3} sx={{ mt: 4 }}>
              {featureItems.map((item) => (
                <Box key={item.number} className={styles.featureItem}>
                  <Box className={styles.featureNumber}>{item.number}</Box>
                  <Box>
                    <Typography variant="h6" className={styles.featureItemTitle}>
                      {item.title}
                    </Typography>
                    <Typography variant="body2" className={styles.featureItemDesc}>
                      {item.description}
                    </Typography>
                  </Box>
                </Box>
              ))}
            </Stack>
          </Box>

          {/* Right: CTA card */}
          <Box className={styles.ctaCard}>
            <Box className={styles.ctaIconWrapper}>
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="32" height="32" fill="none" stroke="white" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                <circle cx="12" cy="12" r="10" />
                <circle cx="12" cy="12" r="6" />
                <circle cx="12" cy="12" r="2" />
              </svg>
            </Box>
            <Typography variant="h5" className={styles.ctaCardTitle}>
              Need Help Right Away?
            </Typography>
            <Typography variant="body2" className={styles.ctaCardDesc}>
              Our expert support team is available 24/7 to assist you with any critical issues or questions.
            </Typography>
            <Stack spacing={2} sx={{ width: "100%", mt: 2 }}>
              <Button
                variant="contained"
                color="warning"
                fullWidth
                component="a"
                href="#"
                target="_blank"
                rel="noopener noreferrer"
                sx={{ textTransform: "none", fontWeight: 600, fontSize: "1rem", paddingY: 1.5, borderRadius: "50px" }}
              >
                Access Customer Portal
              </Button>
              <Button
                variant="outlined"
                fullWidth
                component="a"
                href="https://wso2.com/contact"
                target="_blank"
                rel="noopener noreferrer"
                sx={{ textTransform: "none", fontWeight: 600, fontSize: "1rem", paddingY: 1.5, borderRadius: "50px", "&:hover": { backgroundColor: "rgba(255,103,0,0.08)" } }}
              >
                Contact Us
              </Button>
            </Stack>
          </Box>
        </Box>
      </Box>

      {/* Footer */}
      <Footer />
    </Box>
  );
}
