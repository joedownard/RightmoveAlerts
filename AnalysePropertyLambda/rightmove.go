package main

import (
	"github.com/gocolly/colly/v2"
	"strconv"
	"strings"
)

func GetListingDetailsFromPropertyId(id int64) (RightmoveListingDetails, error) {
	c := colly.NewCollector()

	listingDetails := RightmoveListingDetails{}

	var description strings.Builder

	c.OnHTML("._2uGNfP4v5SSYyfx3rZngKM img", func(e *colly.HTMLElement) {
		listingDetails.ImageURL = e.Attr("src")
	})

	c.OnHTML("._2uQQ3SV0eMHL1P6t5ZDo2q", func(e *colly.HTMLElement) {
		listingDetails.Title = e.Text
	})

	c.OnHTML("._1gfnqJ3Vtd1z40MlC0MzXu", func(e *colly.HTMLElement) {
		listingDetails.Rent = e.DOM.Children().First().Text()
	})

	c.OnHTML("._21Dc_JVLfbrsoEkZYykXK5", func(e *colly.HTMLElement) {
		description.WriteString(e.Text)
	})

	c.OnHTML("._4hBezflLdgDMdFtURKTWh", func(e *colly.HTMLElement) {
		description.WriteString(e.Text)
	})

	c.OnHTML("article[data-testid=\"primary-layout\"]", func(e *colly.HTMLElement) {
		description.WriteString(e.Text)
	})

	c.OnHTML("._2CdMEPuAVXHxzb5evl1Rb8", func(e *colly.HTMLElement) {
		description.WriteString(e.Text)
	})

	listingDetails.Link = "https://www.rightmove.co.uk/properties/" + strconv.FormatInt(id, 10)

	err := c.Visit(listingDetails.Link)
	if err != nil {
		return RightmoveListingDetails{}, err
	}

	listingDetails.Description = description.String()

	return listingDetails, nil
}
